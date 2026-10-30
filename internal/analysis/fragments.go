package analysis

import (
	"encoding/binary"
	"hash/fnv"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Fragment is a syntax subtree that Duplication compares across the
// codebase. Two fragments with the same Shape are structural clones: equal
// once identifiers and literals are abstracted. Lines are 1-based.
type Fragment struct {
	Shape     uint64
	StartLine int
	EndLine   int
}

// Fragments parses source in the given language and returns every named
// subtree with tokens on at least minLines lines. A token is a leaf, or an
// identifier or literal, which counts on its first line only.
func Fragments(source []byte, lang Language, minLines int) ([]Fragment, error) {
	tree, err := parse(source, lang)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	t := tokens{}
	var fragments []Fragment
	var visit func(n *sitter.Node) (Fragment, span)
	visit = func(n *sitter.Node) (Fragment, span) {
		h := fnv.New64a()
		h.Write([]byte(n.Kind()))
		h.Write([]byte{0})
		start, end := lines(n)
		f := Fragment{StartLine: start, EndLine: end}
		var s span
		switch {
		case lang.Abstracted[n.Kind()]:
			// Only the kind counts: renaming or changing the value keeps the shape.
			s = t.add(start)
		case n.ChildCount() == 0:
			h.Write([]byte(n.Utf8Text(source)))
			s = t.add(start)
		default:
			// The fragment covers only the children it compares.
			s = span{first: -1}
			var buf [8]byte
			for i := uint(0); i < n.ChildCount(); i++ {
				if lang.IgnoredByDuplication[n.Child(i).Kind()] {
					continue
				}
				child, cs := visit(n.Child(i))
				binary.LittleEndian.PutUint64(buf[:], child.Shape)
				h.Write(buf[:])
				if cs.first < 0 {
					continue
				}
				if s.first < 0 {
					s.first, f.StartLine = cs.first, child.StartLine
				}
				s.last, f.EndLine = cs.last, child.EndLine
			}
		}
		f.Shape = h.Sum64()
		if n.IsNamed() && s.first >= 0 && t.lines(s) >= minLines {
			fragments = append(fragments, f)
		}
		return f, s
	}
	visit(tree.RootNode())
	return fragments, nil
}

// tokens records the line of each token in source order.
type tokens struct {
	line []int
	// distinct[i] is the number of distinct lines among tokens 0 to i.
	distinct []int
}

// span is a run of tokens, by index; first is -1 for none.
type span struct{ first, last int }

func (t *tokens) add(line int) span {
	d := 1
	if n := len(t.line); n > 0 {
		d = t.distinct[n-1]
		if t.line[n-1] != line {
			d++
		}
	}
	t.line = append(t.line, line)
	t.distinct = append(t.distinct, d)
	i := len(t.line) - 1
	return span{i, i}
}

// lines counts the distinct lines the tokens in s lie on.
func (t *tokens) lines(s span) int {
	return t.distinct[s.last] - t.distinct[s.first] + 1
}

// lines returns the 1-based line range n covers. A node ending at the start
// of a line, such as a statement's newline terminator, does not cover that line.
func lines(n *sitter.Node) (start, end int) {
	s, e := n.StartPosition(), n.EndPosition()
	start, end = int(s.Row)+1, int(e.Row)+1
	if e.Column == 0 && e.Row > s.Row {
		end--
	}
	return start, end
}
