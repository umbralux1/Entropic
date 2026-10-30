package cli_test

import (
	"testing"

	"github.com/umbralux1/entropic/internal/testrepo"
)

func TestMixedGoAndPythonRepoReportsBothLanguagesUnits(t *testing.T) {
	repo := testrepo.New(t).
		Write("main.go", `package main

func main() {
	for {
	}
}
`). // main 1; 5 non-blank lines
		Write("shop/__init__.py", "").
		Write("shop/cart.py", `class Cart:
    def total(self, items):
        return sum(i.price for i in items if i.price)  # +1 for, +1 if


def empty(cart):
    if not cart:  # +1
        return True
    return False
`). // Cart.total 2, empty 1; 7 non-blank lines
		Commit("add code")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	cases := []struct {
		kind, name, module string
		complexity, lines  int
	}{
		{"function", "main", ".", 1, 4},
		{"function", "Cart.total", "shop", 2, 2},
		{"function", "empty", "shop", 1, 4},
		{"class", "Cart", "shop", 2, 3},
		{"file", "shop/cart.py", "shop", 3, 7},
		{"module", "shop", "", 3, 7},
		{"repository", "", "", 4, 12},
	}
	for _, c := range cases {
		u := r.Unit(t, c.kind, c.name)
		if u.Complexity != c.complexity || u.Lines != c.lines || u.Module != c.module {
			t.Errorf("%s %q: want module %q, complexity %d, lines %d; got %+v",
				c.kind, c.name, c.module, c.complexity, c.lines, u)
		}
	}
	if c := r.Unit(t, "class", "Cart"); c.Path != "shop/cart.py" || c.StartLine != 1 || c.EndLine != 3 {
		t.Errorf("Cart: want shop/cart.py lines 1-3, got %+v", c)
	}
}

func TestPythonModulesAreDirectories(t *testing.T) {
	repo := testrepo.New(t).
		Write("app/models/user.py", "def name():\n    pass\n").
		Write("scripts/run.py", "def run():\n    pass\n"). // a directory, not a package
		Commit("add code")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	for _, m := range []string{"app/models", "scripts"} {
		r.Unit(t, "module", m)
	}
	if got := len(r.UnitsOfKind("module")); got != 2 {
		t.Errorf("want 2 Modules, got %+v", r.UnitsOfKind("module"))
	}
}

func TestDeletingOneOfTwoSameNamedPythonMethodsMarksOnlyThatMethodRemoved(t *testing.T) {
	repo := testrepo.New(t).Write("shapes.py", `class A:
    def __init__(self):
        self.x = 1


class B:
    def __init__(self):
        if True:
            self.x = 2
`).Commit("add shapes").Write("shapes.py", `class A:
    pass


class B:
    def __init__(self):
        if True:
            self.x = 2
`)

	r := repo.Run("--format", "json").Report(0)

	if a := r.Unit(t, "function", "A.__init__"); a.Status != "removed" {
		t.Errorf("A.__init__: want removed, got %+v", a)
	}
	if b := r.Unit(t, "function", "B.__init__"); b.Status != "unchanged" {
		t.Errorf("B.__init__: want unchanged, got %+v", b)
	}
	if fns := r.UnitsOfKind("function"); len(fns) != 2 {
		t.Errorf("want exactly A.__init__ and B.__init__, got %+v", fns)
	}
}
