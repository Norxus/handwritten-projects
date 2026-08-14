package schema

import (
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/cloudwego/eino/internal/safe"
)

var ErrNoValue = errors.New("no value")

var ErrRecvAfterClosed = errors.New("recv after stream closed")

type SourceEOF struct {
	sourceName string
}

func (e *SourceEOF) Error() string {
	return fmt.Sprintf("EOF from source stream: %s", e.sourceName)
}

func GetSourceName(err error) (string, bool) {
	var sErr *SourceEOF
	if errors.As(err, &sErr) {
		return sErr.sourceName, true
	}
	return "", false
}

func Pipe[T any](cap int) (*StreamReader[T], *StreamWriter[T]) {
	stm := newStream[T](cap)
	return stm.asReader(), &StreamWriter[T]{stm: stm}
}

// 在 stream 外面包了一层，用于装饰，底层调用的还是 stream 的 send 方法
type StreamWriter[T any] struct {
	stm *stream[T]
}

func (sw *StreamWriter[T]) Send(chunk T, err error) (closed bool) {
	return sw.stm.send(chunk, err)
}

func (sw *StreamWriter[T]) Close() {
	sw.stm.closeSend()
}

type readerType int

const (
	readerTypeStream readerType = iota
	readerTypeArray
	readerTypeMultiStream
	readerTypeWithConvert
	readerTypeChild
)

type stream[T any] struct {
	items  chan streamItem[T]
	closed chan struct{}

	automaticClose bool
	closedFlag     *uint32
}

type streamItem[T any] struct {
	chunk T
	err   error
}

type arrayReader[T any] struct {
	arr   []T
	index int
}

// 把 []T 包装成一个“按次读取”的 reader
func StreamReaderFromArray[T any](arr []T) *StreamReader[T] {
	return &StreamReader[T]{ar: &arrayReader[T]{arr: arr}, typ: readerTypeArray}
}

// 从数组中取值
func (ar *arrayReader[T]) recv() (T, error) {
	if ar.index < len(ar.arr) {
		ret := ar.arr[ar.index]
		ar.index++

		return ret, nil
	}

	var t T
	return t, io.EOF
}

// 把原数组复制 n 份，只是复制了指针，但是底层数据还是同一份
func (ar *arrayReader[T]) copy(n int) []*arrayReader[T] {
	ret := make([]*arrayReader[T], n)

	for i := 0; i < n; i++ {
		ret[i] = &arrayReader[T]{
			arr:   ar.arr,
			index: ar.index,
		}
	}

	return ret
}

func (ar *arrayReader[T]) toStream() *stream[T] {
	return arrToStream[T](ar.arr[ar.index:])
}

// 把所有数据都缓存进 stream 数据源
func arrToStream[T any](arr []T) *stream[T] {
	// 初始化长度为 len(arr) 的 chan
	s := newStream[T](len(arr))
	for i := range arr {
		s.send(arr[i], nil)
	}
	s.closeSend()

	return s
}

func (sr *StreamReader[T]) toStream() *stream[T] {
	switch sr.typ {
	case readerTypeStream:
		return sr.st
	case readerTypeArray:
		return sr.ar.toStream()
	case readerTypeMultiStream:
		return sr.msr.toStream()
	case readerTypeWithConvert:
		return sr.srw.toStream()
	case readerTypeChild:
		return sr.csr.toStream()
	default:
		panic("impossible")
	}
}

type multiStreamReader[T any] struct {
	sts []*stream[T]

	// 动态 select 分支，使用时 reflect.Select(itemsCases)
	// “动态 select ”本质就是“把原本写在 select { case ... } 里的分支，改成用数据结构在运行时表示出来”。
	itemCases []reflect.SelectCase

	// 存 sts 的下标
	nonClosed []int

	// 保存每个源 StreamReader 的名字
	sourceReaderNames []string
}

func newMultiStreamReader[T any](sts []*stream[T]) *multiStreamReader[T] {
	var itemsCases []reflect.SelectCase
	// 小数量 channel（1 到 5 个）时，直接用手写 select ，更轻量
	// 大数量的 channel 用动态的方式
	if len(sts) > maxSelectNum {
		itemsCases = make([]reflect.SelectCase, len(sts))
		for i, st := range sts {
			// 第 i 个 case 是一个接收操作，接收的 channel 是 st.items
			itemsCases[i] = reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(st.items),
			}
		}
	}

	// 初始化未关闭的 channel
	nonClosed := make([]int, len(sts))
	for i := range sts {
		nonClosed[i] = i
	}

	return &multiStreamReader[T]{
		sts:       sts,
		itemCases: itemsCases,
		nonClosed: nonClosed,
	}
}

func (msr *multiStreamReader[T]) recv() (T, error) {
	for len(msr.nonClosed) > 0 {
		var chosen int
		var ok bool

		// reflect 是运行时反射，开销比直接写死的 select 大
		// 直接 select 是编译器原生支持的路径，没有这些反射层
		if len(msr.nonClosed) > maxSelectNum {
			var recv reflect.Value
			chosen, recv, ok := reflect.Select(msr.itemCases)
			if ok {
				item := recv.Interface().(streamItem[T])
				return item.chunk, item.err
			}
			msr.itemCases[chosen].Chan = reflect.Value{}
		} else {
			var item *streamItem[T]
			chosen, item, ok = receiveN(msr.nonClosed, msr.sts)
			if ok {
				return item.chunk, item.err
			}
		}

		// ok == false 时才会走到这里，说明 channel 已经关闭了
		// 已关闭的子流从活跃列表里删掉
		for i := range msr.nonClosed {
			if msr.nonClosed[i] == chosen {
				msr.nonClosed = append(msr.nonClosed[:i], msr.nonClosed[i+1:]...)
			}
		}

		if len(msr.sourceReaderNames) > 0 {
			var t T
			return t, &SourceEOF{msr.sourceReaderNames[chosen]}
		}
	}

	var t T
	return t, io.EOF
}

func (msr *multiStreamReader[T]) nonClosedStreams() []*stream[T] {
	ret := make([]*stream[T], len(msr.nonClosed))

	for i, idx := range msr.nonClosed {
		ret[i] = msr.sts[idx]
	}

	return ret
}

// 把所有的 stream 依次关闭
func (msr *multiStreamReader[T]) close() {
	for _, s := range msr.sts {
		s.closeRecv()
	}
}

func (msr *multiStreamReader[T]) toStream() *stream[T] {
	return toStream[T, *multiStreamReader[T]](msr)
}

type iStreamReader interface {
	recvAny() (any, error)
	copyAny(int) []iStreamReader
	Close()
	SetAutomaticClose()
}

type streamReaderWithConvert[T any] struct {
	sr         iStreamReader
	convert    func(any) (T, error)
	errWrapper func(error) error
}

type convertOptions struct {
	ErrWrapper func(error) error
}

type ConvertOption func(*convertOptions)

func newStreamReaderWithConvert[T any](origin iStreamReader, convert func(any) (T, error), opts ...ConvertOption) *StreamReader[T] {
	opt := &convertOptions{}
	// 这里可能设置好 errWrapper
	for _, o := range opts {
		o(opt)
	}

	srw := &streamReaderWithConvert[T]{
		sr:         origin,
		convert:    convert,
		errWrapper: opt.ErrWrapper,
	}

	return &StreamReader[T]{
		typ: readerTypeWithConvert,
		srw: srw,
	}
}

func WithErrWrapper(wrapper func(error) error) ConvertOption {
	return func(o *convertOptions) {
		o.ErrWrapper = wrapper
	}
}

// 使用 convert 转换获取的每一个元素
func StreamReaderWithConvert[T, D any](sr *StreamReader[T], convert func(T) (D, error), opts ...ConvertOption) *StreamReader[D] {
	// 把转换函数包装一下，这样才可以使用 newStreamReaderWithConvert 基建
	c := func(a any) (D, error) {
		return convert(a.(T))
	}

	return newStreamReaderWithConvert(sr, c, opts...)
}

func (srw *streamReaderWithConvert[T]) recv() (T, error) {
	for {
		// 依赖的是内部的 istreamReader
		out, err := srw.sr.recvAny()

		if err != nil {
			var t T
			if err == io.EOF {
				return t, err
			}

			// 有指定的错误包装器，那么包装错误并返回
			if srw.errWrapper != nil {
				err = srw.errWrapper(err)
				if err != nil {
					return t, err
				}

				continue
			}

			return t, err
		}

		// 内部的 convert 函数对输出进行转换
		t, err := srw.convert(out)
		if err == nil {
			return t, nil
		}

		//  如果只是 ErrNoValue，跳过这条数据，继续读下一条
		if !errors.Is(err, ErrNoValue) {
			return t, err
		}
	}
}

func (srw *streamReaderWithConvert[T]) close() {
	srw.sr.Close()
}

func (srw *streamReaderWithConvert[T]) toStream() *stream[T] {
	return toStream[T, *streamReaderWithConvert[T]](srw)
}

type cpStreamElement[T any] struct {
	once sync.Once
	next *cpStreamElement[T]
	item streamItem[T]
}

type parentStreamReader[T any] struct {
	sr *StreamReader[T]

	subStreamList []*cpStreamElement[T]

	closedNum uint32
}

type childStreamReader[T any] struct {
	parent *parentStreamReader[T]
	index  int
}

// 装饰器，统一分发接口
type StreamReader[T any] struct {
	typ readerType

	st *stream[T]

	ar *arrayReader[T]

	msr *multiStreamReader[T]

	srw *streamReaderWithConvert[T]

	csr *childStreamReader[T]
}

// 委托给内部不同类型的 reader 去调用 receive()
func (sr *StreamReader[T]) Recv() (T, error) {
	switch sr.typ {
	case readerTypeStream:
		return sr.st.recv()
	case readerTypeArray:
		return sr.ar.recv()
	case readerTypeMultiStream:
		return sr.msr.recv()
	case readerTypeWithConvert:
		return sr.srw.recv()
	case readerTypeChild:
		return sr.csr.recv()
	default:
		panic("impossible")
	}
}

func (sr *StreamReader[T]) Close() {
	switch sr.typ {
	case readerTypeStream:
		sr.st.closeRecv()
	case readerTypeArray:
		// array 没有 close
	case readerTypeMultiStream:
		sr.msr.close()
	case readerTypeWithConvert:
		sr.srw.close()
	case readerTypeChild:
		sr.csr.close()
	default:
		panic("impossible")
	}
}

// 把一个输入流“分叉”成 n 个独立的 StreamReader
// 这 n 个 reader 都会收到原始流里的每一个元素
func (sr *StreamReader[T]) Copy(n int) []*StreamReader[T] {
	if n < 2 {
		// 如果 n 小于 2，就不真的复制，直接把原始 sr 放进切片返回
		return []*StreamReader[T]{sr}
	}

	if sr.typ == readerTypeArray {
		ret := make([]*StreamReader[T], n)
		for i, ar := range sr.ar.copy(n) {
			ret[i] = &StreamReader[T]{typ: readerTypeArray, ar: ar}
		}
		return ret
	}

	return copyStreamReaders[T](sr, n)
}

// 把一个原始 StreamReader 包装成 n 个“子 reader”
// 这些子 reader 表面上彼此独立，但它们背后共享同一个“父级调度器”，由这个父级统一从原始流里取数据并分发给每个子 reader
func copyStreamReaders[T any](sr *StreamReader[T], n int) []*StreamReader[T] {
	// 使用传入的 reader 制作基 reader
	cpsr := &parentStreamReader[T]{
		sr:            sr,
		subStreamList: make([]*cpStreamElement[T], n),
		closedNum:     0,
	}

	// 站位节点
	elem := &cpStreamElement[T]{}

	// 对于与每一个子 reader 读取进度
	// 所有子 reader 一开始都指向同一个空节点，多个 child 要先共享一个一致的起点
	// 之所以一开始 subStreamList[i] 不能都是 nil，因为两种含义冲突了
	// 这个 child 还没开始读，这个 child 已经关闭了，但后面的实现里， nil 明确被拿来表示“这个 child 已关闭”
	for i := range cpsr.subStreamList {
		cpsr.subStreamList[i] = elem
	}

	// 制作 childReader, 把基 reader 放入 parent 位置并返回
	ret := make([]*StreamReader[T], n)
	for i := range ret {
		ret[i] = &StreamReader[T]{
			csr: &childStreamReader[T]{
				parent: cpsr,
				index:  i,
			},
			typ: readerTypeChild,
		}
	}

	return ret
}

func (sr *StreamReader[T]) SetAutomaticClose() {
	switch sr.typ {
	case readerTypeStream:
		// 给 StreamReader 开启“自动兜底关闭”能力 ，避免调用方忘记手动 Close() 时把底层流资源挂住。
		if !sr.st.automaticClose {
			sr.st.automaticClose = true

			// 这里赋值为 0 表示还没有被关闭过，因为后面 close 的时候会进行 CAS(0, 1) 的操作
			// 只有开启自动关闭才需要这个 flag，因为只有这种情况下才需要考虑重复关闭的问题
			var flag uint32
			sr.st.closedFlag = &flag
			// 给 sr 挂一个 GC finalizer。也就是当 sr 这个对象之后不再可达、准备被 GC 回收时，运行时会尝试调用 s.Close()
			runtime.SetFinalizer(sr, func(s *StreamReader[T]) {
				s.Close()
			})
		}
	case readerTypeMultiStream:
		// 相比于上面只是多了一个 for 循环
		for _, s := range sr.msr.nonClosedStreams() {
			if !s.automaticClose {
				s.automaticClose = true
				var flag uint32
				s.closedFlag = &flag
				runtime.SetFinalizer(s, func(st *stream[T]) {
					st.closeRecv()
				})
			}
		}
	case readerTypeChild:
		// 把自动关闭能力设置到它背后的父 reader 上
		// readerTypeChild 是 Copy(n) 产生出来的子 reader，真正的数据源不是它自己，而是父级 parentStreamReader 持有的原始 sr
		parent := sr.csr.parent.sr
		parent.SetAutomaticClose()
	case readerTypeWithConvert:
		sr.srw.sr.SetAutomaticClose()
	case readerTypeArray:
		// no need to clean up
	default:
	}
}

func (sr *StreamReader[T]) recvAny() (any, error) {
	return sr.Recv()
}

func (sr *StreamReader[T]) copyAny(n int) []iStreamReader {
	ret := make([]iStreamReader, n)

	// 返回类型是 []*StreamReader[T]
	// 需要包装一下返回
	srs := sr.Copy(n)

	for i := 0; i < n; i++ {
		ret[i] = srs[i]
	}

	return ret
}

// 让第 idx 个 child reader 读取它当前这一个位置的元素
// 如果这个位置还没真正从上游取过数据，就由它负责取一次
// 后其他 child 再读到同一个位置时，直接复用这次结果
func (p *parentStreamReader[T]) peek(idx int) (t T, err error) {
	elem := p.subStreamList[idx]
	// 如果这里是 nil ，说明这个 child 已经关闭了
	if elem == nil {
		return t, ErrRecvAfterClosed
	}

	// 当其他 child 走到这里的时候，由于这个 elem 的 once 已经被使用过了，所以会
	// 直接跳过
	elem.once.Do(func() {
		// 第一个到达这个节点的 child，会真的调用 p.sr.Recv()
		t, err = p.sr.Recv()
		// 给空节点填入数据
		elem.item = streamItem[T]{chunk: t, err: err}
		// 如果没有到 EOF，就顺手创建下一个空节点
		// 当前节点 elem 被填成一个“真实数据节点”以后，再挂出一个新的空节点 elem.next
		// 这个新空节点代表“下一个位置的数据以后填这里”
		if err != io.EOF {
			// 指向下一个之后，说明该 child 的 sync.once 使用权也刷新了
			elem.next = &cpStreamElement[T]{}
			p.subStreamList[idx] = elem.next
		}
	})

	// 所有 child 都从当前节点里拿到同一份结果，直接拿缓存的结果
	t = elem.item.chunk
	err = elem.item.err
	// 同时也把指针往后挪到同样的进度，流结束就不用创建新节点了
	if err != io.EOF {
		p.subStreamList[idx] = elem.next
	}

	return t, err
}

func (p *parentStreamReader[T]) close(idx int) {
	// 已经关闭直接退出
	if p.subStreamList[idx] == nil {
		return
	}

	// 置为空着暴雨关注
	p.subStreamList[idx] = nil

	// 关闭数目 + 1
	curClosedNum := atomic.AddUint32(&p.closedNum, 1)

	allClosed := int(curClosedNum) == len(p.subStreamList)
	if allClosed {
		// 如果所有 child 都关了，说明再也没有人需要这个原始上游流
		// 这时就把底层原始 sr 也关闭，释放资源
		p.sr.Close()
	}
}

// 读取委托给父对象
func (csr *childStreamReader[T]) recv() (T, error) {
	return csr.parent.peek(csr.index)
}

func (csr *childStreamReader[T]) toStream() *stream[T] {
	// 注意，这里是 *childStreamReader[T]，而不是 childStreamReader[T]
	// childStreamReader[T] 的 recv() 和 close() 都是 指针接收者方法 ，
	// 所以实现 reader[T] 接口的是 *childStreamReader[T] ，不是 childStreamReader[T]
	// 平时调用方法可以不用管这个区别，是因为编译器自动做了相关的转换，但这里不一样，T 的方法集 实际上和 *T 的方法集并不一样
	return toStream[T, *childStreamReader[T]](csr)
}

func (csr *childStreamReader[T]) close() {
	csr.parent.close(csr.index)
}

func newStream[T any](cap int) *stream[T] {
	return &stream[T]{
		items:  make(chan streamItem[T], cap),
		closed: make(chan struct{}),
	}
}

func (s *stream[T]) asReader() *StreamReader[T] {
	return &StreamReader[T]{typ: readerTypeStream, st: s}
}

// 直接从 channel 中拿
func (s *stream[T]) recv() (chunk T, err error) {
	item, ok := <-s.items

	if !ok {
		item.err = io.EOF
	}

	return item.chunk, item.err
}

func (s *stream[T]) send(chunk T, err error) (closed bool) {
	// 快速失败
	select {
	case <-s.closed:
		return true
	default:
	}

	item := streamItem[T]{chunk, err}

	// 看消息和 closed 哪个先出现就走哪个分支
	select {
	case <-s.closed:
		return true
	case s.items <- item:
		return false
	}
}

func (s *stream[T]) closeSend() {
	// 直接关闭通道
	close(s.items)
}

func (s *stream[T]) closeRecv() {
	// 如果 closeRecv() 可能被调用多次，尤其可能并发调用，就应该为 true
	if s.automaticClose {
		// 只有当 closedFlag 还是 0 时，才把它原子地改成 1
		// 也就是说 只有“第一个成功改成 1 的调用者”才真正执行 close(s.closed)
		// 这是因为 如果对已经关闭的 channel 再 close ，程序会 panic
		// 避免手动关闭和自动关闭重复
		if atomic.CompareAndSwapUint32(s.closedFlag, 0, 1) {
			close(s.closed)
		}
		return
	}
	close(s.closed)
}

type reader[T any] interface {
	recv() (T, error)
	close()
}

// 起一个 goroutine，持续从传入的 reader 里调用 recv() 拿数据，再转发到返回的 *stream[T] 里
// 从 r Reader 把数据发送到 *stream[T] 中
// T any：定义了一个类型参数 T，any 表示 T 可以是任意类型
// Reader reader[T]：定义了第二个类型参数 Reader，这个类型参数不能随便取，必须满足约束 reader[T]
// 之所以不写成 func toStream[T any](r reader[T]) *stream[T]，是为了留下具体类型信息
func toStream[T any, Reader reader[T]](r Reader) *stream[T] {
	// chan buffer 为 5
	ret := newStream[T](5)

	// 构建生产者协程
	go func() {
		defer func() {
			// 捕获 goroutine 内的 panic
			panicErr := recover()
			if panicErr != nil {
				// 把 panic 包装成普通 error
				e := safe.NewPanicErr(panicErr, debug.Stack())

				// 让下游像处理普通数据一样，处理 panic
				var chunk T
				_ = ret.send(chunk, e)
			}

			// 通知消费端，这个流不会再有新数据了
			ret.closeSend()
			// 释放底层 reader 持有的资源
			r.close()
		}()

		for {
			// 从 reader 拉取一条数据
			out, err := r.recv()
			if err == io.EOF {
				// 没数据了，break
				break
			}

			// 把数据发送过去
			closed := ret.send(out, err)
			if closed {
				break
			}
		}
	}()

	return ret
}

// 合并一堆 stream reader 成为一个 stream reader
func MergeStreamReaders[T any](srs []*StreamReader[T]) *StreamReader[T] {
	if len(srs) < 1 {
		return nil
	}

	if len(srs) < 2 {
		return srs[0]
	}

	var arr []T
	var ss []*stream[T]

	// 按照不同类型分开处理
	for _, sr := range srs {
		switch sr.typ {
		case readerTypeStream:
			ss = append(ss, sr.st)
		case readerTypeArray:
			// 把数组里还没读完的数据都放到 arr 中
			arr = append(arr, sr.ar.arr[sr.ar.index:]...)
		case readerTypeMultiStream:
			// 把 multi stream 里的所有还没关闭的子流都放到 ss 中
			ss = append(ss, sr.msr.nonClosedStreams()...)
		case readerTypeWithConvert:
			// 转换成普通 stream
			ss = append(ss, sr.srw.toStream())
		case readerTypeChild:
			ss = append(ss, sr.csr.toStream())
		default:
			panic("impossible")
		}
	}

	// 说明全是数组类型
	if len(ss) == 0 {
		return &StreamReader[T]{
			typ: readerTypeArray,
			ar: &arrayReader[T]{
				arr:   arr,
				index: 0,
			},
		}
	}

	// 数组类型和 stream 类型都有，就把数组类型转换为 stream 类型
	if len(arr) != 0 {
		s := arrToStream(arr)
		ss = append(ss, s)
	}

	// 最后合并为一个 multi stream reader
	return &StreamReader[T]{
		typ: readerTypeMultiStream,
		msr: newMultiStreamReader(ss),
	}
}

// 把一组带名字的 StreamReader[T] 合并成一个 StreamReader[T]
func InternalMergeNamedStreamReaders[T any](srs []*StreamReader[T], names []string) *StreamReader[T] {
	ss := make([]*stream[T], len(srs))
	// 把 reader 转换为 stream 类型
	for i, sr := range srs {
		ss[i] = sr.toStream()
	}

	//记录每个 reader 的名字
	msr := newMultiStreamReader(ss)
	msr.sourceReaderNames = names

	return &StreamReader[T]{
		typ: readerTypeMultiStream,
		msr: msr,
	}
}
