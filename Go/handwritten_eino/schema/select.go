package schema

const maxSelectNum = 5

// 把长度为 1~4 的 select 情况，每种都硬编码出来
// - hosenList []int ：保存“被选中的流在 ss 里的下标”
// - ss []*stream[T] ：保存所有可读取的 stream
func receiveN[T any](chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
	// 定义了一个切片 []func(chosenList []int, ss []*stream[T]) (index int, item *streamItem[T], ok bool)
	// 从中取 len(chosenList) 的元素，发起调用
	return []func(chosenList []int, ss []*stream[T]) (index int, item *streamItem[T], ok bool){
		nil,
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			item, ok := <-ss[chosenList[0]].items
			return chosenList[0], &item, ok
		},
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			select {
			case item, ok := <-ss[chosenList[0]].items:
				return chosenList[0], &item, ok
			case item, ok := <-ss[chosenList[1]].items:
				return chosenList[1], &item, ok
			}
		},
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			select {
			case item, ok := <-ss[chosenList[0]].items:
				return chosenList[0], &item, ok
			case item, ok := <-ss[chosenList[1]].items:
				return chosenList[1], &item, ok
			case item, ok := <-ss[chosenList[2]].items:
				return chosenList[2], &item, ok
			}
		},
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			select {
			case item, ok := <-ss[chosenList[0]].items:
				return chosenList[0], &item, ok
			case item, ok := <-ss[chosenList[1]].items:
				return chosenList[1], &item, ok
			case item, ok := <-ss[chosenList[2]].items:
				return chosenList[2], &item, ok
			case item, ok := <-ss[chosenList[3]].items:
				return chosenList[3], &item, ok
			}
		},
		func(chosenList []int, ss []*stream[T]) (int, *streamItem[T], bool) {
			select {
			case item, ok := <-ss[chosenList[0]].items:
				return chosenList[0], &item, ok
			case item, ok := <-ss[chosenList[1]].items:
				return chosenList[1], &item, ok
			case item, ok := <-ss[chosenList[2]].items:
				return chosenList[2], &item, ok
			case item, ok := <-ss[chosenList[3]].items:
				return chosenList[3], &item, ok
			case item, ok := <-ss[chosenList[4]].items:
				return chosenList[4], &item, ok
			}
		},
	}[len(chosenList)](chosenList, ss)
}
