package core

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/internal/generic"
)

type AddressSegmentType string

type Address []AddressSegment

func (p Address) String() string {
	if p == nil {
		return ""
	}

	var sb strings.Builder
	for i, s := range p {
		sb.WriteString(string(s.Type))
		sb.WriteString(":")
		sb.WriteString(s.ID)
		if s.SubID != "" {
			sb.WriteString(":")
			sb.WriteString(s.SubID)
		}
		if i != len(p)-1 {
			sb.WriteString(";")
		}
	}
	return sb.String()
}

func (p Address) Equals(other Address) bool {
	if len(p) != len(other) {
		return false
	}

	for i := range p {
		if p[i].Type != other[i].Type || p[i].ID != other[i].ID || p[i].SubID != other[i].SubID {
			return false
		}
	}

	return true
}

type AddressSegment struct {
	ID    string
	Type  AddressSegmentType
	SubID string
}

type addrCtxKey struct{}

type addrCtx struct {
	addr           Address
	interruptState *InterruptState
	isResumeTarget bool
	resumeData     any
}

type globalResumeInfoKey struct{}

type globalResumeInfo struct {
	mu sync.Mutex
	// 恢复数据
	id2ResumeData map[string]any
	// 哪些恢复数据被消费过
	id2ResumeDataUsed map[string]bool
	id2State          map[string]InterruptState
	id2StateUsed      map[string]bool
	id2Addr           map[string]Address
}

// 从 ctx 中获取 graph 中执行到哪里了，比如处在某个 graph node 下的某个 tool call
func GetCurrentAddress(ctx context.Context) Address {
	if p, ok := ctx.Value(addrCtx{}).(*addrCtx); ok {
		return p.addr
	}
	return nil
}

// 把当前的位置信息追加到地址末尾
func AppendAddressSegment(ctx context.Context, segType AddressSegmentType, segID string, subID string) context.Context {
	// 获取当前的具体位置
	currentAddress := GetCurrentAddress(ctx)
	if len(currentAddress) == 0 {
		currentAddress = []AddressSegment{
			{
				Type:  segType,
				ID:    segID,
				SubID: subID,
			},
		}
	} else {
		newAddress := make([]AddressSegment, len(currentAddress)+1)
		// currentAdress -> newAddress
		copy(newAddress, currentAddress)
		newAddress[len(newAddress)-1] = AddressSegment{
			Type:  segType,
			ID:    segID,
			SubID: subID,
		}

		currentAddress = newAddress
	}

	runCtx := &addrCtx{
		addr: currentAddress,
	}

	// 从 ctx 中拿 globalResumeInfo 信息
	rInfo, hasRInfo := getResumeInfo(ctx)
	if !hasRInfo {
		return context.WithValue(ctx, addrCtxKey{}, runCtx)
	}

	var id string
	// id_ 并非特殊写法。只是为了区分 id
	for id_, addr := range rInfo.id2Addr {
		// 查找一个和当前 currentAddress 相同的历史地址
		if addr.Equals(currentAddress) {
			rInfo.mu.Lock()
			// 如果这个地址对应的状态还没被用过，就把那份状态挂到当前 runCtx.interruptState 上，并标记为“已消费”
			if used, ok := rInfo.id2StateUsed[id_]; !ok || !used {
				runCtx.interruptState = generic.PtrOf(rInfo.id2State[id_])
				// 把地址状态修改为已消费
				rInfo.id2StateUsed[id_] = true
				id = id_
				rInfo.mu.Unlock()
				break
			}
			rInfo.mu.Unlock()
		}
	}

	// 继续加锁
	rInfo.mu.Lock()
	defer rInfo.mu.Unlock()

	used := rInfo.id2ResumeDataUsed[id]
	if !used {
		// 这份消费数据没有被使用过
		rData, existed := rInfo.id2ResumeData[id]
		// 使用这份消费数据
		if existed {
			rInfo.id2ResumeDataUsed[id] = true
			runCtx.resumeData = rData
			runCtx.isResumeTarget = true
		}
	}

	// 当前节点没有被认定为恢复目标
	if !runCtx.isResumeTarget {
		for id_, addr := range rInfo.id2Addr {
			// - addr 是不是 currentAddress 的一个“更深层后代地址”，eg A/B/C 和 A/B
			if len(addr) > len(currentAddress) && addr[:len(currentAddress)].Equals(currentAddress) {
				// 如果发现任意“某个后代地址”的 resumeData 还没被消费，就把当前 runCtx 标记为恢复目标
				// 子债父偿
				if !rInfo.id2ResumeDataUsed[id_] {
					runCtx.isResumeTarget = true
					break
				}
			}
		}
	}

	return context.WithValue(ctx, addrCtxKey{}, runCtx)
}

func getResumeInfo(ctx context.Context) (*globalResumeInfo, bool) {
	info, ok := ctx.Value(globalResumeInfoKey{}).(*globalResumeInfo)
	return info, ok
}

func GetNextResumptionPoints(ctx context.Context) (map[string]bool, error) {
	parentAddr := GetCurrentAddress(ctx)

	rInfo, exists := getResumeInfo(ctx)
	if !exists {
		return nil, fmt.Errorf("GetNextResumptioPoints: failed to get resume info from context")
	}

	nextPoints := make(map[string]bool)
	parentAddrLen := len(parentAddr)

	for _, addr := range rInfo.id2Addr {

		// 不是子地址，直接跳过
		if len(addr) <= parentAddrLen {
			continue
		}

		var isPrefix bool
		if parentAddrLen == 0 {
			isPrefix = true
		} else {
			isPrefix = addr[:parentAddrLen].Equals(parentAddr)
		}

		if !isPrefix {
			continue
		}

		// 毗邻父地址的就是子地址
		childAddr := addr[parentAddrLen : parentAddrLen+1]
		childID := childAddr[0].ID

		if _, ok := nextPoints[childID]; !ok {
			nextPoints[childID] = true
		}

	}
	return nextPoints, nil
}

func BatchResumeWithData(ctx context.Context, resumeData map[string]any) context.Context {
	rInfo, ok := ctx.Value(globalResumeInfoKey{}).(*globalResumeInfo)
	if !ok {
		// 浅拷贝一个避免后面的改动影响
		newMap := make(map[string]any, len(resumeData))
		for k, v := range resumeData {
			newMap[k] = v
		}

		// 没有就加上
		return context.WithValue(ctx, globalResumeInfoKey{}, &globalResumeInfo{
			id2ResumeData:     newMap,
			id2ResumeDataUsed: make(map[string]bool),
			id2StateUsed:      make(map[string]bool),
		})
	}

	rInfo.mu.Lock()
	defer rInfo.mu.Unlock()

	if rInfo.id2ResumeData == nil {
		rInfo.id2ResumeData = make(map[string]any)
	}
	// 合并新的 resumeData
	for id, data := range resumeData {
		rInfo.id2ResumeData[id] = data
	}

	return ctx
}

func PopulateInterruptState(ctx context.Context, id2Addr map[string]Address, id2State map[string]InterruptState) context.Context {
	rInfo, ok := ctx.Value(globalResumeInfoKey{}).(*globalResumeInfo)
	// 有就合并，没有就新建
	if ok {
		if rInfo.id2Addr == nil {
			rInfo.id2Addr = make(map[string]Address)
		}
		// 合并地址信息
		for id, addr := range id2Addr {
			rInfo.id2Addr[id] = addr
		}
		rInfo.id2State = id2State
	} else {
		rInfo = &globalResumeInfo{
			id2Addr:           id2Addr,
			id2State:          id2State,
			id2StateUsed:      make(map[string]bool),
			id2ResumeDataUsed: make(map[string]bool),
		}
		ctx = context.WithValue(ctx, globalResumeInfoKey{}, rInfo)
	}

	runCtx, ok := getRunCtx(ctx)
	if ok {
		for id_, addr := range id2Addr {
			// 找出当前地址
			if addr.Equals(runCtx.addr) {
				// 如果对应的状态没有被消费过
				if used, ok := rInfo.id2StateUsed[id_]; !ok || !used {
					runCtx.interruptState = generic.PtrOf(rInfo.id2State[id_])
					rInfo.mu.Lock()
					rInfo.id2StateUsed[id_] = true
					rInfo.mu.Unlock()
				}

				// 如果恢复数据没有被消费过，标记成“这次 resume 的目标节点”，并拿到恢复时传入的数据
				if used, ok := rInfo.id2ResumeDataUsed[id_]; !ok || !used {
					runCtx.isResumeTarget = true
					runCtx.resumeData = rInfo.id2ResumeData[id_]
					rInfo.mu.Lock()
					rInfo.id2ResumeDataUsed[id_] = true
					rInfo.mu.Unlock()
				}

				break
			}
		}
	}

	return ctx
}

type InterruptInfo struct {
	Info        any
	IsRootCause bool
}

func (i *InterruptInfo) String() string {
	if i == nil {
		return ""
	}
	return fmt.Sprintf("interrupt info: Info=%v, IsRootCause=%v", i.Info, i.IsRootCause)
}
