package generic

import "reflect"

// 1. 获取 T 得反射类型
// 2. 根据反射类型创建实例
func NewInstance[T any]() T {
	typ := TypeOf[T]()

	switch typ.Kind() {
	case reflect.Map:
		return reflect.MakeMap(typ).Interface().(T)
	case reflect.Slice, reflect.Array:
		return reflect.MakeSlice(typ, 0, 0).Interface().(T)
	case reflect.Ptr:
		// 指针指向的类型, eg: **int, 此时 typ 是 *int 类型
		typ = typ.Elem()
		// origin 也是一个指针 (**int 类型)，指向的是指针指向的类型的实例, 也就是执行了 new(*int)
		origin := reflect.New(typ)
		inst := origin

		// 类似于快慢指针的思想， 想象一个洋葱，inst 是慢指针，指向洋葱外层，typ 是快指针，指向洋葱内层
		// 从外往里一层层构造出整个洋葱，inst 指向已经存在的层，typ 指向不存在的层
		// 然后通过 reflect.New(typ) 构造出不存在的层，并通过 inst.Set(reflect.New(typ)) 建立内外层之间的关系
		// type 用于指向 inst 指针指向的类型
		// 然后为 inst 填充 type 类型的实例类型，直到 type 类型不是指针类型
		// 循环做的事情就是 ****MyStruct -> ***MyStruct -> nil
		// ****MyStruct -> ***MyStruct -> **MyStruct -> nil
		// ****MyStruct -> ***MyStruct -> **MyStruct -> *MyStruct -> nil
		// ****MyStruct -> ***MyStruct -> **MyStruct -> *MyStruct -> MyStruct
		for typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
			inst = inst.Elem()
			// 要理解 reflect.New(typ) 是做什么的，假如 reflect.New(**MyStruct)
			// 它是分配一个变量，这个变量的类型是 **MyStruct ，初始值是该类型的零值 nil ，然后返回这个变量的地址。
			inst.Set(reflect.New(typ))
		}

		return origin.Interface().(T)
	default:
		var t T
		return t
	}
}

/*
TypeOf 获取泛型参数 T 对应的反射类型 reflect.Type。

================================================================================
一、为什么不直接写 reflect.TypeOf(*new(T))，而要用 (*T)(nil) + Elem() 的惯用法？
================================================================================

核心原因：需要支持"接口类型"作为 T 的情况。

 1. 接口类型只存在于编译期的静态检查阶段，运行期不存在"接口类型"这种东西。
 2. 运行期，从接口值里只能反查出"动态类型"和"动态值"；因为同一个值可能实现多个接口。
 3. 运行期只记录"它真的是什么"（动态类型），不记录"它被当作什么看待"（静态类型）。
    静态类型纯粹是编译器视角下的视图。
 4. reflect.TypeOf 的签名是 func(i any) Type。任何值传进去都会先被装箱进 interface{}，
    TypeOf 再从这个 interface{} 中读出动态类型返回。
    —— 这就意味着：如果 T 本身是 io.Writer 这种接口类型，装箱过程会丢失接口类型信息。

================================================================================
二、"具体类型" 与 "接口类型" 的关键区别
================================================================================

具体类型的判定标准只有一条：

		这个类型的值，在编译期能不能确定它的精确内存布局？

	  - 具体类型：内存布局完全确定，编译期就知道占几个字节、字段排布、方法实现。
	    例如 int、struct{...}、*error（指针本身就是 8 字节，毫无歧义）。

	  - 接口类型：可以包含多种不同的具体类型，大小不确定，布局无法在编译期确定。
	    所以接口值必须在运行期携带类型信息。

	  - 接口类型的指针（如 *io.Writer）：仍然是具体类型。
	    指针本身是 8 字节；"指针指向什么类型"这个信息由编译器记录在符号表里，
	    运行期 CPU 看到的只是裸地址，没有类型概念。

================================================================================
三、接口值的运行期表示（iface / itab）
================================================================================

接口值不直接指向具体类型描述符，而是指向 itab；具体类型描述符放在 itab 里：

	iface {
	    tab  *itab      // 指向 (接口类型, 具体类型) 这一对儿的 itab
	    data pointer    // 指向实际数据
	}

	itab {
	    inter *interfacetype  // 接口类型描述符，如 io.Writer
	    _type *_type          // 具体类型描述符，如 *os.File  ← 动态类型在这里
	    fun   [N]uintptr      // 预先排好序的方法地址表
	}

以 w io.Writer = &os.File{...} 为例的内存结构：

	w (io.Writer 接口值)
	│
	├── tab  ───→ itab
	│              ├── inter  ───→ io.Writer 类型描述符
	│              ├── _type  ───→ *os.File 类型描述符   ← 具体类型在这
	│              └── fun    ───→ [Write 地址, Close 地址, ...]
	│
	└── data ───→ 实际的 *os.File 数据

调用 w.Write(...) 时：
 1. 拿到 tab
 2. 直接跳 tab.fun[i]（编译期就知道 Write 在第 i 槽）
 3. 一次跳转完成

为什么要引入 itab，而不是让 tab 直接指向具体类型描述符？

	因为具体类型（如 *os.File）可能有几十上百个方法，而接口（如 io.Writer）只关心其中一个。
	如果每次调用都在大方法列表里线性查找，既慢又无法利用 CPU 分支预测。
	itab 把接口关心的那几个方法按固定顺序预先排好，调用时直接按下标索引。

================================================================================
四、为什么 reflect.TypeOf(w) 拿不到 io.Writer？
================================================================================

  - itab 里有 inter 字段（接口类型描述符），说明接口值确实"知道自己是什么接口"。
  - 但 inter 只在 itab 里活着，不会透出到最外层的接口值中。
  - reflect.TypeOf 的参数类型是 any（即 interface{}）。w 传进去时会发生"接口转接口"装箱：
    编译器只把 itab._type（具体类型）复制到新的 any.eface._type 里，inter 丢失。
  - 所以 reflect.TypeOf(w) 看到的永远是 *os.File，而不是 io.Writer。

要拿到 io.Writer 这个接口类型本身，必须绕开装箱机制 ——
这正是 reflect.TypeOf((*io.Writer)(nil)).Elem() 惯用法存在的根源。

================================================================================
五、(*T)(nil) + Elem() 惯用法为什么能工作？
================================================================================

核心思路：不去从接口值中获取接口类型，而是通过接口指针指向的接口类型描述符中获取。

背景知识：类型描述符住在哪？
  - Go 二进制文件里静态链接了一堆类型描述符，它们由编译器在编译期生成。
  - 程序跑起来后，这些描述符真实存在于内存的 .rodata 段（只读数据段）。
  - 结论：类型描述符 = 编译期产生的、运行期只读的"数据"。
  - reflect.Type 在底层就是 *rtype —— 指向类型描述符的指针。
  - reflect.TypeOf(x) 做的事：从 x 这个 interface{} 的 _type 字段里读出指针，
    包装成 reflect.Type 返回。本质上就是"拿到一个指向二进制某块只读内存的指针"。

下面以 reflect.TypeOf((*io.Writer)(nil)).Elem() 为例拆解：

【编译期发生的事】
 1. 编译器看到 *io.Writer 出现在代码里。
 2. 编译器为 *io.Writer 生成一份"指针类型描述符"烧进二进制；
    其 elem 字段指向 io.Writer 的接口类型描述符。
 3. 编译器为 io.Writer 也生成一份"接口类型描述符"（若尚未生成）。
    注：同一个类型在整个程序里只生成一份描述符，多处使用共享同一地址
    —— 这是编译器 + 链接器协同实现的去重优化。
 4. 编译器把 (*io.Writer)(nil) 编译成：
    "一个 nil 指针，类型标签是 io.Writer 类型描述符的地址"。
 5. 装箱进 any 时，生成代码：
    eface._type = &type:*io.Writer
    eface.data  = nil
    注意：这些都是"数据"而不是"代码"，会以静态数据形式烧进二进制。

【运行期发生的事】
 1. 执行 reflect.TypeOf(...)，传入的 any 值已经是 {_type: &type:*io.Writer, data: nil}。
 2. TypeOf 读 _type 字段，拿到指针，包装成 reflect.Type 返回
    —— 这是一个指向 .rodata 段里 *io.Writer 描述符的引用。
 3. 调用 .Elem()：反射读 *io.Writer 描述符里的 elem 字段，
    得到 io.Writer 接口类型描述符的指针，再包装成 reflect.Type。

整个运行期过程都在"读编译期写好的只读数据"，没有任何信息凭空出现，
但这些类型信息确实在运行期可访问。

================================================================================
六、总结
================================================================================
(*T)(nil) 的本质：绕开"接口值"这条通道，直接引用"类型描述符"本身。
这样无论 T 是具体类型还是接口类型，都能拿到 T 准确的反射类型信息。
*/
func TypeOf[T any]() reflect.Type {
	// (*T)(nil) 是为了获取一个值为 nil，类型为 *T 的变量
	// 之所以是 *T，而不是 T，是因为 T 有可能不能为 nil，比如 int 类型等等
	return reflect.TypeOf((*T)(nil)).Elem()
}

func PtrOf[T any](v T) *T {
	return &v
}

type Pair[F, S any] struct {
	First  F
	Second S
}

// S ~[]E 表示 S 不仅匹配直接的 []E ，还匹配“底层类型是 []E 的自定义类型”
// 比如 type MyInts []int
func Reverse[S ~[]E, E any](s S) S {
	d := make(S, len(s))
	for i := 0; i < len(s); i++ {
		d[i] = s[len(s)-i-1]
	}
	return d
}

func CopyMap[K comparable, V any](src map[K]V) map[K]V {
	dst := make(map[K]V, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
