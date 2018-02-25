package rbtree

import (
	"errors"
	"unsafe"
)

var (
	ErrNotInTree  = errors.New("node is not a node of this tree")
	ErrNoLast     = errors.New("begin of tree has no Last()")
	ErrNoNext     = errors.New("end of tree has no Next()")
	ErrEraseEmpty = errors.New("can't erase empty node")
)

const _NodeSize = unsafe.Sizeof(node{})
const _NodeOffSet = unsafe.Offsetof(struct {
	child1 node
	child2 node
	parent node
	color  colorType
}{}.color)
const _PointerSize = unsafe.Sizeof(unsafe.Pointer(nil))
const _DefaultMaxSpan = 512
const _ColorSize = unsafe.Sizeof(colorType(false))

type colorType bool

const (
	red   = false
	black = true
)

type Node struct {
	node
	tree *tree
}

type node struct {
	i int32
	j int32
}

func (n Node) GetData() interface{} {
	return n.tree.getData(n.node)
}

func (n Node) GetKey() interface{} {
	return n.tree.getKey(n.node)
}

func (n Node) GetVal() interface{} {
	return n.tree.getVal(n.node)
}

func (n Node) SetVal(val interface{}) {
	n.tree.setVal(n.node, val)
}

func (n Node) Next() node {
	return n.tree.next(n.node)
}

func (n Node) Last() node {
	return n.tree.last(n.node)
}

type mem struct {
	p    unsafe.Pointer
	size uintptr
}

type tree struct {
	header  node
	keyType *_type
	valType *_type
	size    int
	compare func(a, b interface{}) int
	unique  bool
	// true means data is interface, so spans will only store the pointer of key or value
	// then the directkey and directval must be false
	ifacedata bool
	// directkey is true will store the key direct in spans
	directkey bool
	// directval is true will store the value direct in spans
	directval bool
	// maxSpan means the max number of node alloc to a span of spans
	maxSpan int32
	// curSpan means the current number of node alloc to a span of spans
	curSpan uintptr
	keySize uintptr
	valSize uintptr
	// spans is the memory to store node data, key and value
	// it arrange in this way:
	// maxSpan*(child [2]node,parent node),maxSpan*key,maxSpan*val,maxSpan*(color colorType)
	// color also can store as a bit
	spans []mem
	// freeNodes store the node free by deleteNode
	// use two-dimension slice to avoid a too long append action in a tree action
	// when there is no free slice to free node, alloc a slice whose len is curSpan
	freeNodes [][]node
}

func (t *tree) Init(unique bool, key, val interface{}, compare func(a, b interface{}) int) {
	t.unique = unique
	t.keyType = unpackIface(key)._type
	t.valType = unpackIface(val)._type
	t.size = 1
	t.compare = compare
	t.spans = nil
	t.freeNodes = nil
	t.maxSpan = _DefaultMaxSpan

	if t.keyType == nil {
		panic("no key type")
	}
	t.directkey = !isDirectIface(t.keyType) && !t.ifacedata
	if t.directkey {
		t.keySize = t.keyType.size
	} else {
		t.keySize = _PointerSize
	}
	t.valSize = 0
	if t.valType != nil {
		t.directval = !isDirectIface(t.valType) && !t.ifacedata
		if t.directval {
			t.valSize = t.valType.size
		} else {
			t.valSize = _PointerSize
		}
	}
	t.header = t.newNode(key, val)
	t.setChild(t.header, 0, t.end())
	t.setChild(t.header, 1, t.end())
	t.setParent(t.header, t.end())
	t.setColor(t.header, red)
}

func (t *tree) SetMaxSpan(maxSpan int) {
	t.maxSpan = int32(maxSpan)
}

func (t *tree) getChild(n node, ch uintptr) node {
	return *t.getChildPointer(n, ch)
}

func (t *tree) getChildPointer(n node, ch uintptr) *node {
	return (*node)(add(t.spans[n.i].p, uintptr(n.j)*_NodeOffSet+ch*_PointerSize))
}

func (t *tree) setChild(n node, ch uintptr, child node) {
	*t.getChildPointer(n, ch) = child
}

func (t *tree) getParent(n node) node {
	return *t.getChildPointer(n, 2)
}

func (t *tree) getParentPointer(n node) *node {
	return t.getChildPointer(n, 2)
}

func (t *tree) setParent(n node, parent node) {
	*t.getChildPointer(n, 2) = parent
}

func (t *tree) getColor(n node) colorType {
	return *t.getColorPointer(n)
}

func (t *tree) setColor(n node, color colorType) {
	*t.getColorPointer(n) = color
}

func (t *tree) getColorPointer(n node) *colorType {
	offset := t.spans[n.i].size*(_NodeOffSet+t.keySize+t.valSize) + uintptr(n.j)*_ColorSize
	return (*colorType)(add(t.spans[n.i].p, offset))
}

func (t *tree) getKeyPointer(n node) unsafe.Pointer {
	return add(t.spans[n.i].p, t.spans[n.i].size*_NodeOffSet+uintptr(n.j)*t.keySize)
}

func (t *tree) getValPointer(n node) unsafe.Pointer {
	return add(t.spans[n.i].p, t.spans[n.i].size*(_NodeOffSet+t.keySize)+uintptr(n.j)*t.valSize)
}

func (t *tree) getKey(n node) interface{} {
	kp := t.getKeyPointer(n)
	if !t.directkey {
		kp = *(*unsafe.Pointer)(kp)
	}
	return pack2Iface(t.keyType, kp)
}

func (t *tree) getVal(n node) interface{} {
	if t.valType == nil {
		panic("no value")
	}
	vp := t.getValPointer(n)
	if !t.directval {
		vp = *(*unsafe.Pointer)(vp)
	}
	return pack2Iface(t.valType, vp)
}

func (t *tree) setKey(node node, key interface{}) {
	keyEface := unpackIface(key)
	if keyEface._type != t.keyType {
		panic("not same key type")
	}
	if t.directkey {
		memcopy(t.getKeyPointer(node), keyEface.p, t.keySize)
	} else {
		*(*unsafe.Pointer)(t.getKeyPointer(node)) = keyEface.p
	}
}

func (t *tree) setVal(node node, val interface{}) {
	valEface := unpackIface(val)
	if valEface._type != t.valType {
		panic("not same value type")
	}
	if t.directval {
		memcopy(t.getValPointer(node), valEface.p, t.valType.size)
	} else {
		*(*unsafe.Pointer)(t.getValPointer(node)) = valEface.p
	}
}

func (t *tree) getData(node node) interface{} {
	return t.getKey(node)
}

func (t *tree) copyNodeData(des, src node) {
	memcopy(t.getKeyPointer(des), t.getKeyPointer(src), t.keySize)
	if t.valType != nil {
		memcopy(t.getValPointer(des), t.getValPointer(src), t.valSize)
	}
}

func (t *tree) newNode(key, val interface{}) node {
	if len(t.freeNodes) <= 0 {
		t.curSpan = uintptr(t.size)
		if t.curSpan > uintptr(t.maxSpan) {
			t.curSpan = uintptr(t.maxSpan)
		} else if t.size <= 8 {
			t.curSpan = 8 // begin at 8 node, and then the curSpan must be the multiple of 8
		}
		t.spans = append(t.spans, mem{
			p:    newmem(t.curSpan * (_NodeOffSet + t.keySize + t.valSize + _ColorSize)),
			size: t.curSpan})
		nodes := make([]node, 0, t.curSpan)
		for i := uintptr(0); i < t.curSpan; i++ {
			nodes = append(nodes, node{int32(len(t.spans)) - 1, int32(i)})
		}
		t.freeNodes = append(t.freeNodes, nodes)
	}
	n := t.freeNodes[0][0]
	t.freeNodes[0] = t.freeNodes[0][1:]
	if len(t.freeNodes[0]) == 0 {
		t.freeNodes = t.freeNodes[1:]
	}
	t.initNode(n)
	t.size++
	return n
}

func (t *tree) initNode(n node) {
	t.setChild(n, 0, t.end())
	t.setChild(n, 1, t.end())
	t.setParent(n, t.end())
	t.setColor(n, red)
}

func (t *tree) deleteNode(n node) {
	memclr(t.getKeyPointer(n), t.keySize)
	if t.valType != nil {
		memclr(t.getValPointer(n), t.valSize)
	}
	t.size--
	l := len(t.freeNodes)
	if l <= 0 || cap(t.freeNodes[l-1]) == len(t.freeNodes[l-1]) {
		nodes := make([]node, 0, t.curSpan)
		t.freeNodes = append(t.freeNodes, nodes)
	}
	l = len(t.freeNodes)
	t.freeNodes[l-1] = append(t.freeNodes[l-1], n)
}

func (t *tree) pack(n node) Node {
	return Node{node: n, tree: t}
}

func (t *tree) Size() int {
	return t.size
}

func (t *tree) Unique() bool {
	return t.unique
}

func (t *tree) Empty() bool {
	return t.size == 0
}

func (t *tree) Begin() Node {
	return t.pack(t.begin())
}

func (t *tree) begin() node {
	return t.most(0)
}

func (t *tree) End() Node {
	return t.pack(t.end())
}

func (t *tree) end() node {
	return t.header
}

func (t *tree) mustGetColor(n node) colorType {
	if !sameNode(n, t.end()) {
		return t.getColor(n)
	}
	return black
}

func (t *tree) root() node {
	return t.getParent(t.header)
}

func (t *tree) rootPoiter() *node {
	return t.getParentPointer(t.header)
}

//ch = 0: leftmost; ch = 1: rightmost
func (t *tree) most(ch uintptr) node {
	return t.getChild(t.header, ch)
}

//ch = 0: leftmostPoiter; ch = 1: rightmostPoiter
func (t *tree) mostPoiter(ch uintptr) *node {
	return t.getChildPointer(t.header, ch)
}

func sameNode(a, b node) bool {
	return a == b
}

// Next return the next Node of n in this tree
// if n has no next Node, it will panic
func (t *tree) Next(n Node) Node {
	if t != n.tree {
		panic(ErrNotInTree)
	}
	return t.pack(t.next(n.node))
}
func (t *tree) next(n node) node {
	if sameNode(n, t.end()) {
		panic(ErrNoNext)
	}
	if sameNode(n, t.most(1)) {
		return t.end()
	}
	return t.gothrough(1, n)
}

// Last return the last Node of n in this tree
// if n has no last Node, it will panic
func (t *tree) Last(n Node) Node {
	if t != n.tree {
		panic(ErrNotInTree)
	}
	return t.pack(t.last(n.node))
}
func (t *tree) last(n node) node {
	if sameNode(n, t.begin()) {
		panic(ErrNoLast)
	}
	if sameNode(n, t.end()) {
		return t.most(1)
	}
	return t.gothrough(0, n)
}

func (t *tree) gothrough(ch uintptr, n node) node {
	if !sameNode(t.getChild(n, ch), t.end()) {
		n = t.getChild(n, ch)
		for !sameNode(t.getChild(n, ch^1), t.end()) {
			n = t.getChild(n, ch^1)
		}
		return n
	}
	for !sameNode(t.getParent(n), t.end()) && sameNode(t.getChild(t.getParent(n), ch), n) {
		n = t.getParent(n)
	}
	return t.getParent(n)
}

// Count return the num of n key equal to key in this tree
func (t *tree) Count(key interface{}) (count int) {
	if t.unique {
		if sameNode(t.find(key), t.end()) {
			return 0
		}
		return 1
	}
	var beg = t.lowerBound(key)
	for !sameNode(beg, t.end()) && t.compare(t.getKey(beg), key) == 0 {
		beg = t.next(beg)
		count++
	}
	return count
}

// EqualRange return the Node range of equal key n in this tree
func (t *tree) EqualRange(key interface{}) (beg, end Node) {
	return t.LowerBound(key), t.UpperBound(key)
}

// Find return the Node of key in this tree
// if the key is not exist in this tree, result will be the End of tree
// if there has multi n key equal to key, result will be random one
func (t *tree) Find(key interface{}) Node {
	return t.pack(t.find(key))
}
func (t *tree) find(key interface{}) node {
	var root = t.root()
	for {
		if sameNode(root, t.end()) {
			return root
		}
		switch cmp := t.compare(key, t.getKey(root)); {
		case cmp == 0:
			return root
		case cmp < 0:
			root = t.getChild(root, 0)
		case cmp > 0:
			root = t.getChild(root, 1)
		}
	}
}

// Insert insert a new n with data to tree
// it return the insert n Node and true when success insert
// otherwise, it return the end of tree and false
func (t *tree) Insert(key, val interface{}) (Node, bool) {
	n, ok := t.insert(key, val)
	return t.pack(n), ok
}
func (t *tree) insert(key, val interface{}) (node, bool) {
	var root = t.root()
	var rootPoiter = t.rootPoiter()
	if sameNode(root, t.end()) {
		t.size++
		*rootPoiter = t.newNode(key, val)
		t.insertAdjust(*rootPoiter)
		*t.mostPoiter(0) = *rootPoiter
		*t.mostPoiter(1) = *rootPoiter
		return *rootPoiter, true
	}
	var parent = t.getParent(root)
	for !sameNode(root, t.end()) {
		parent = root
		switch cmp := t.compare(key, t.getKey(root)); {
		case cmp == 0:
			if t.unique {
				return t.end(), false
			}
			fallthrough
		case cmp < 0:
			rootPoiter = t.getChildPointer(root, 0)
			root = *rootPoiter
		case cmp > 0:
			rootPoiter = t.getChildPointer(root, 1)
			root = *rootPoiter
		}
	}
	t.size++
	*rootPoiter = t.newNode(key, val)
	t.setParent((*rootPoiter), parent)
	for ch := uintptr(0); ch < 2; ch++ {
		if sameNode(parent, t.most(ch)) && sameNode(t.getChild(parent, ch), *rootPoiter) {
			*t.mostPoiter(ch) = *rootPoiter
		}
	}
	t.insertAdjust(*rootPoiter)
	return *rootPoiter, true
}

//insert n is default red
func (t *tree) insertAdjust(n node) {
	var parent = t.getParent(n)
	if sameNode(parent, t.end()) {
		//fmt.Println("case 1: insert")
		//n is root,set black
		t.setColor(n, black)
		return
	}
	if t.getColor(parent) == black {
		//fmt.Println("case 2: insert")
		//if parent is black,do nothing
		return
	}

	//parent is red,grandpa can't be empty and color is black
	var grandpa = t.getParent(parent)
	var parentCh uintptr = 0
	if sameNode(t.getChild(grandpa, 1), parent) {
		parentCh = 1
	}

	var uncle = t.getChild(grandpa, parentCh^1)
	if !sameNode(uncle, t.end()) && t.getColor(uncle) == red {
		//fmt.Println("case 3: insert")
		//uncle is red
		t.setColor(parent, black)
		t.setColor(grandpa, red)
		t.setColor(uncle, black)
		t.insertAdjust(grandpa)
		return
	}

	var childCh uintptr = 0
	if sameNode(t.getChild(parent, 1), n) {
		childCh = 1
	}
	if childCh != parentCh {
		//fmt.Println("case 4: insert")
		t.rotate(parentCh, n)
		var tmp = parent
		parent = n
		n = tmp
	}

	//fmt.Println("case 5: insert")
	t.setColor(parent, black)
	t.setColor(grandpa, red)
	t.rotate(parentCh^1, parent)
}

// Erase erase all the n keys equal to key in this tree and return the number of erase n
func (t *tree) Erase(key interface{}) (count int) {
	var keyPointer = noescape(interface2pointer(key))
	if t.unique {
		var iter = t.find(keyPointer)
		if sameNode(iter, t.end()) {
			return 0
		}
		t.eraseNode(iter)
		return 1
	}
	var beg = t.lowerBound(key)
	for !sameNode(beg, t.end()) && t.compare(keyPointer, t.getKeyPointer(beg)) == 0 {
		var tmp = t.next(beg)
		t.eraseNode(beg)
		beg = tmp
		count++
	}
	return count
}

// EraseNode erase n from the tree
// if n is not in tree, it will panic
func (t *tree) EraseNode(n Node) {
	if t != n.tree {
		panic(ErrNotInTree)
	}
	t.eraseNode(n.node)
}
func (t *tree) eraseNode(n node) {
	if sameNode(n, t.end()) {
		panic(ErrEraseEmpty)
	}
	t.size--
	if !sameNode(t.getChild(n, 0), t.end()) && !sameNode(t.getChild(n, 1), t.end()) {
		//if n has two child,it's last n must has no more than one child,copy to n and erase last n
		var tmp = t.last(n)
		t.copyNodeData(n, tmp)
		n = tmp
	}
	//adjust leftmost and rightmost
	for ch := uintptr(0); ch < 2; ch++ {
		if sameNode(t.most(ch), n) {
			if ch == 0 {
				*t.mostPoiter(ch) = t.next(n)
			} else {
				*t.mostPoiter(ch) = t.last(n)
			}
		}
	}
	var child = t.end()
	if !sameNode(t.getChild(n, 0), t.end()) {
		child = t.getChild(n, 0)
	} else if !sameNode(t.getChild(n, 1), t.end()) {
		child = t.getChild(n, 1)
	}
	var parent = t.getParent(n)
	if !sameNode(child, t.end()) {
		t.setParent(child, parent)
	}
	if sameNode(parent, t.end()) {
		*t.rootPoiter() = child
	} else if sameNode(t.getChild(parent, 0), n) {
		t.setChild(parent, 0, child)
	} else {
		t.setChild(parent, 1, child)
	}
	if t.getColor(n) == black { //if n is red,just erase,otherwise adjust
		t.eraseAdjust(child, parent)
		//fmt.Println("eraseAdjust:")
	}
	t.deleteNode(n)
	return
}

func (t *tree) eraseAdjust(n, parent node) {
	if sameNode(parent, t.end()) {
		//n is root
		//fmt.Println("case 1: erase")
		if !sameNode(n, t.end()) {
			t.setColor(n, black)
		}
		return
	}
	if t.mustGetColor(n) == red {
		//n is red,just set black
		//fmt.Println("case 2: erase")
		t.setColor(n, black)
		return
	}
	var nCh uintptr = 0
	if sameNode(t.getChild(parent, 1), n) {
		nCh = 1
	}
	var brother = t.getChild(parent, nCh^1)
	//after case 1 parent must not be empty n and after case 2 n must be black
	if t.getColor(parent) == red {
		//parent is red,brother must be black but can't be empty n,because the path has a black n more
		if t.mustGetColor(t.getChild(brother, 0)) == black && t.mustGetColor(t.getChild(brother, 1)) == black {
			//fmt.Println("case 3: erase")
			t.setColor(brother, red)
			t.setColor(parent, black)
			return
		}
		if !sameNode(brother, t.end()) && t.mustGetColor(t.getChild(brother, nCh)) == red {
			//fmt.Println("case 4: erase", nCh)
			t.setColor(parent, black)
			t.rotate(nCh^1, t.getChild(brother, nCh))
			t.rotate(nCh, t.getChild(parent, nCh^1))
			return
		}
		//fmt.Println("case 5: erase")
		t.rotate(nCh, brother)
		return
	}
	//parent is black
	if t.mustGetColor(brother) == red {
		//brother is red, it's children must be black
		//fmt.Println("case 6: erase")
		t.setColor(brother, black)
		t.setColor(parent, red)
		t.rotate(nCh, brother)
		t.eraseAdjust(n, parent) //goto redParent then end
		return
	}
	//brother is black
	if t.mustGetColor(t.getChild(brother, 0)) == black && t.mustGetColor(t.getChild(brother, 1)) == black {
		//fmt.Println("case 7: erase")
		t.setColor(brother, red)
		t.eraseAdjust(parent, t.getParent(parent))
		return
	}
	if t.mustGetColor(t.getChild(brother, nCh)) == red {
		//fmt.Println("case 8: erase", nCh)
		t.setColor(t.getChild(brother, nCh), black)
		t.rotate(nCh^1, t.getChild(brother, nCh))
		t.rotate(nCh, t.getChild(parent, nCh^1))
		return
	}
	//fmt.Println("case 9: erase", nCh)
	t.setColor(t.getChild(brother, nCh^1), black)
	t.rotate(nCh, brother)
}

// EraseNodeRange erase the given iterator range
// if the given range is not in this tree, it will panic with ErrNoIntree
// if end can get beg after multi Next method, it will panic with ErrNoLast
func (t *tree) EraseNodeRange(beg, end Node) (count int) {
	return t.eraseNodeRange(beg.node, end.node)
}
func (t *tree) eraseNodeRange(beg, end node) (count int) {
	for !sameNode(beg, end) {
		var tmp = t.next(beg)
		t.eraseNode(beg)
		beg = tmp
		count++
	}
	return count
}

// LowerBound return the first Node greater than or equal to key
func (t *tree) LowerBound(key interface{}) Node {
	return t.pack(t.lowerBound(key))
}
func (t *tree) lowerBound(key interface{}) node {
	var root = t.root()
	var parent = t.end()
	for {
		if root == t.end() {
			if sameNode(parent, t.end()) {
				return parent
			} else if t.compare(key, t.getKey(parent)) <= 0 {
				return parent
			}
			return t.next(parent)
		}
		parent = root
		if t.compare(key, t.getKey(root)) > 0 {
			root = t.getChild(root, 1)
		} else {
			root = t.getChild(root, 0)
		}
	}
}

// UpperBound return the first Node greater than key
func (t *tree) UpperBound(key interface{}) Node {
	return t.pack(t.upperBound(key))
}
func (t *tree) upperBound(key interface{}) node {
	var root = t.root()
	var parent = t.end()
	for {
		if root == t.end() {
			if sameNode(parent, t.end()) {
				return parent
			} else if t.compare(key, t.getKey(parent)) < 0 {
				return parent
			}
			return t.next(parent)
		}
		parent = root
		if t.compare(key, t.getKey(root)) >= 0 {
			root = t.getChild(root, 1)
		} else {
			root = t.getChild(root, 0)
		}
	}
}

//ch = 0:take n for center,left rotate parent down,n is parent's right child
//ch = 1:take n for center,right rotate parent down,n is parent's left child
func (t *tree) rotate(ch uintptr, n node) {
	var (
		tmp     = t.getChild(n, ch)
		parent  = t.getParent(n)
		grandpa = t.getParent(parent)
	)
	t.setChild(n, ch, parent)
	t.setChild(parent, ch^1, tmp)

	if !sameNode(tmp, t.end()) {
		t.setParent(tmp, parent)
	}
	t.setParent(parent, n)
	t.setParent(n, grandpa)
	if sameNode(grandpa, t.end()) {
		*t.rootPoiter() = n
		return
	}
	if sameNode(t.getChild(grandpa, 0), parent) {
		t.setChild(grandpa, 0, n)
	} else {
		t.setChild(grandpa, 1, n)
	}
}
