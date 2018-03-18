package rbtree

type SetNode struct {
	n _node
}

func (n SetNode) GetData() interface{} {
	return n.n.tree.getKey(n.n.node)
}

func (n SetNode) Next() SetNode {
	return SetNode{n.n.tree.Next(n.n)}
}

func (n SetNode) Last() SetNode {
	return SetNode{n.n.tree.Last(n.n)}
}

type Set struct {
	tree
}

func NewSet(data interface{}, compare func(a, b interface{}) int) *Set {
	var s = &Set{}
	s.Init(true, data, compare)
	return s
}

func NewMultiSet(data interface{}, compare func(a, b interface{}) int) *Set {
	var s = &Set{}
	s.Init(false, data, compare)
	return s
}

func (s *Set) pack(n _node) SetNode {
	return SetNode{n: n}
}

func (s *Set) Init(unique bool, data interface{}, compare func(a, b interface{}) int) {
	s.tree.Init(unique, data, nil, compare)
}

func (s *Set) Begin() SetNode {
	return s.pack(s.tree.Begin())
}

func (s *Set) End() SetNode {
	return s.pack(s.tree.End())
}

func (s *Set) EqualRange(data interface{}) (beg, end SetNode) {
	a, b := s.tree.EqualRange(data)
	return s.pack(a), s.pack(b)
}

func (s *Set) EraseNode(n SetNode) {
	s.tree.EraseNode(n.n)
}

func (s *Set) EraseNodeRange(beg, end SetNode) (count int) {
	return s.tree.EraseNodeRange(beg.n, end.n)
}

func (s *Set) Find(data interface{}) SetNode {
	return s.pack(s.tree.Find(data))
}

func (s *Set) Insert(data interface{}) (SetNode, bool) {
	n, ok := s.tree.Insert(data, nil)
	return s.pack(n), ok
}

func (s *Set) LowerBound(data interface{}) SetNode {
	return s.pack(s.tree.LowerBound(data))
}

func (s *Set) Last(n SetNode) SetNode {
	return s.pack(s.tree.Last(n.n))
}

func (s *Set) Next(n SetNode) SetNode {
	return s.pack(s.tree.Next(n.n))
}

func (s *Set) UpperBound(data interface{}) SetNode {
	return s.pack(s.tree.UpperBound(data))
}

func (s *Set) Count(data interface{}) (count int) {
	return s.tree.Count(data)
}

func (s *Set) Erase(data interface{}) (count int) {
	return s.tree.Erase(data)
}
