package analysis_test

import (
	"testing"

	"github.com/umbralux1/entropic/internal/analysis"
	"github.com/umbralux1/entropic/internal/language/python"
)

func TestPythonFunctionsAreUnits(t *testing.T) {
	units, err := analysis.Analyze([]byte(`def a():
    pass


async def b():
    pass


@decorator
def c():
    pass
`), python.Language)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	want := []analysis.Unit{
		{Kind: analysis.Function, Name: "a", StartLine: 1, EndLine: 2, Cyclomatic: 1},
		{Kind: analysis.Function, Name: "b", StartLine: 5, EndLine: 6, Cyclomatic: 1},
		{Kind: analysis.Function, Name: "c", StartLine: 10, EndLine: 11, Cyclomatic: 1},
	}
	if len(units) != len(want) {
		t.Fatalf("want %d units, got %d: %+v", len(want), len(units), units)
	}
	for i, w := range want {
		if units[i] != w {
			t.Errorf("unit %d: want %+v, got %+v", i, w, units[i])
		}
	}
}

func pythonUnits(t *testing.T, source string) []analysis.Unit {
	t.Helper()
	units, err := analysis.Analyze([]byte(source), python.Language)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return units
}

func pythonComplexityOf(t *testing.T, source string) map[string]int {
	t.Helper()
	got := map[string]int{}
	for _, u := range pythonUnits(t, source) {
		got[u.Name] = u.Complexity
	}
	return got
}

func wantComplexity(t *testing.T, got, want map[string]int) {
	t.Helper()
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: want complexity %d, got %d", name, w, got[name])
		}
	}
}

// Campbell, "Cognitive Complexity" (2017), myMethod, with try/catch as
// try/except: loops and branches add 1 plus their nesting. Expected 9.
func TestPythonComplexityPenalisesNesting(t *testing.T) {
	got := pythonComplexityOf(t, `def my_method():
    try:
        if condition1:  # +1
            for i in range(10):  # +2 (nesting 1)
                while condition2:  # +3 (nesting 2)
                    pass
    except (ExcepType1, ExcepType2):  # +1
        if condition2:  # +2 (nesting 1)
            pass
    finally:  # +0
        pass
`)
	wantComplexity(t, got, map[string]int{"my_method": 9})
}

// elif and else add one each with no nesting penalty, but what they contain
// is nested. Hand count: 5.
func TestPythonComplexityElifAndElseHaveNoNestingPenalty(t *testing.T) {
	got := pythonComplexityOf(t, `def grade(x):
    if x > 90:  # +1
        return "a"
    elif x > 80:  # +1
        return "b"
    else:  # +1
        while x > 0:  # +2 (nesting 1)
            x -= 1
        return "c"
`)
	wantComplexity(t, got, map[string]int{"grade": 5})
}

// Campbell, "Cognitive Complexity" (2017), getWords, with switch as match:
// a match counts once, however many cases it has. Expected 1.
func TestPythonComplexityMatchCountsOnce(t *testing.T) {
	got := pythonComplexityOf(t, `def get_words(number):
    match number:  # +1
        case 1:
            return "one"
        case 2:
            return "a couple"
        case 3:
            return "a few"
        case _:
            return "lots"


def nested(command):
    for c in command:  # +1
        match c:  # +2 (nesting 1)
            case "go":
                if c:  # +3 (nesting 2)
                    pass
`)
	wantComplexity(t, got, map[string]int{"get_words": 1, "nested": 6})
}

// Campbell, "Cognitive Complexity" (2017): each sequence of like boolean
// operators adds 1, with no nesting penalty; other operators add none.
func TestPythonComplexityCountsSequencesOfBooleanOperators(t *testing.T) {
	got := pythonComplexityOf(t, `def mixed(a, b, c, d, e, f):
    if a and b and c or d or e and f:  # +1 if, +3 for and, or, and
        pass


def assigned(a, b, c, x):
    ok = a and b and c  # +1
    return ok or (x > 0 and x < 10)  # +2


def negated(a, b, c):
    return a and not (b and c)  # +2: what not negates is a sequence of its own
`)
	wantComplexity(t, got, map[string]int{"mixed": 4, "assigned": 3, "negated": 2})
}

// Campbell, "Cognitive Complexity" (2017), overriddenSymbolFrom, with
// else if as elif. Expected 19.
func TestPythonComplexityOverriddenSymbolFrom(t *testing.T) {
	got := pythonComplexityOf(t, `def overridden_symbol_from(self, class_type):
    if class_type.is_unknown():  # +1
        return Symbols.unknown_method_symbol
    unknown_found = False
    symbols = class_type.get_symbol().members().lookup(self.name)
    for override_symbol in symbols:  # +1
        if (override_symbol.is_kind(JavaSymbol.MTH)  # +2 (nesting 1)
                and not override_symbol.is_static()):  # +1
            method_java_symbol = override_symbol
            if self.can_override(method_java_symbol):  # +3 (nesting 2)
                overriding = self.check_overriding_parameters(method_java_symbol, class_type)
                if overriding is None:  # +4 (nesting 3)
                    if not unknown_found:  # +5 (nesting 4)
                        unknown_found = True
                elif overriding:  # +1
                    return method_java_symbol
    if unknown_found:  # +1
        return Symbols.unknown_method_symbol
    return None
`)
	wantComplexity(t, got, map[string]int{"overridden_symbol_from": 19})
}

// Campbell, "Cognitive Complexity" (2017), myMethod2: a nested function or
// lambda adds nothing itself but nests what is inside it. Expected 2 each.
func TestPythonComplexityNestedFunctionsAndLambdasIncreaseNesting(t *testing.T) {
	got := pythonComplexityOf(t, `def my_method2():
    def r():  # +0 (but nesting level is now 1)
        if condition1:  # +2 (nesting 1)
            pass
    r()


def with_lambda():
    r = lambda: a if condition1 else b  # +2 (nesting 1)
    r()
`)
	wantComplexity(t, got, map[string]int{"my_method2": 2, "with_lambda": 2})
}

// A comprehension's for and if clauses are loops and branches: each adds 1
// plus the nesting the comprehension is at. Hand count in comments.
func TestPythonComplexityComprehensionsCountTheirClauses(t *testing.T) {
	got := pythonComplexityOf(t, `def evens(xs):
    return [x for x in xs if x % 2 == 0]  # +1 for, +1 if


def totals(rows):
    for row in rows:  # +1
        yield {k: v for k, v in row if v}  # +2 for, +2 if (nesting 1)


def flat(grid):
    return list(c for row in grid for c in row)  # +1, +1
`)
	wantComplexity(t, got, map[string]int{"evens": 2, "totals": 5, "flat": 2})
}

// Campbell, "Cognitive Complexity" (2017), sumOfPrimes. Python has no
// labelled continue, so the paper's +1 for continue OUT has no counterpart;
// a loop's else is part of the loop and adds nothing. Expected 7 - 1 = 6.
func TestPythonComplexitySumOfPrimes(t *testing.T) {
	got := pythonComplexityOf(t, `def sum_of_primes(max):
    total = 0
    for i in range(1, max + 1):  # +1
        for j in range(2, i):  # +2 (nesting 1)
            if i % j == 0:  # +3 (nesting 2)
                break  # +0: Python jumps take no label
        else:
            total += i
    return total
`)
	wantComplexity(t, got, map[string]int{"sum_of_primes": 6})
}

// Campbell, "Cognitive Complexity" (2017): recursion adds 1 for each function
// in the recursion cycle, however many recursive calls it makes.
func TestPythonComplexityRecursionIncrements(t *testing.T) {
	got := pythonComplexityOf(t, `def fact(n):
    if n <= 1:  # +1
        return 1
    return n * fact(n - 1)  # +1


class Tree:
    def walk(self, n):
        if n > 0:  # +1
            self.walk(n - 1)  # +1
            self.walk(n - 2)  # +0: already in the cycle

    def delegate(self):
        other.delegate()  # +0: another object's method

    def shadow(self):
        shadow()  # +0: a module function, not this method
`)
	wantComplexity(t, got, map[string]int{"fact": 2, "Tree.walk": 2, "Tree.delegate": 0, "Tree.shadow": 0})
}

// McCabe's cyclomatic complexity: 1 plus each if, elif, loop, except,
// non-wildcard case, conditional expression, comprehension clause and
// boolean operator.
func TestPythonCyclomaticCountsDecisionPoints(t *testing.T) {
	units := pythonUnits(t, `def paths(a, b, xs):
    if a and b or not a:  # +1 if, +2 operators
        pass
    elif b:  # +1
        pass
    else:  # +0
        pass
    while a:  # +1
        pass
    try:
        pass
    except ValueError:  # +1
        pass
    match a:
        case 1:  # +1
            pass
        case _:  # +0
            pass
    ys = [x for x in xs if x]  # +2
    return 1 if a else 2  # +1
`)
	if got := units[0].Cyclomatic; got != 11 {
		t.Errorf("paths: want cyclomatic 11, got %d", got)
	}
}

// Classes are Units too: their Complexity is the roll-up of the methods
// they enclose, nested classes included.
func TestPythonClassesAreUnits(t *testing.T) {
	units := pythonUnits(t, `class Outer:
    def a(self, x):
        if x:  # +1
            pass

    class Inner:
        def b(self, xs):
            for _ in xs:  # +1
                pass
`)
	want := []analysis.Unit{
		{Kind: analysis.Class, Name: "Outer", StartLine: 1, EndLine: 9, Complexity: 2},
		{Kind: analysis.Function, Name: "Outer.a", StartLine: 2, EndLine: 4, Complexity: 1, Cyclomatic: 2},
		{Kind: analysis.Class, Name: "Outer.Inner", StartLine: 6, EndLine: 9, Complexity: 1},
		{Kind: analysis.Function, Name: "Outer.Inner.b", StartLine: 7, EndLine: 9, Complexity: 1, Cyclomatic: 2},
	}
	if len(units) != len(want) {
		t.Fatalf("want %d units, got %d: %+v", len(want), len(units), units)
	}
	for i, w := range want {
		if units[i] != w {
			t.Errorf("unit %d: want %+v, got %+v", i, w, units[i])
		}
	}
}

// Methods are named after their class, so that same-named methods in
// different classes stay distinct. A function nested in another is part of
// the enclosing function, not a Unit of its own.
func TestPythonMethodsAreQualifiedWithTheirClass(t *testing.T) {
	var names []string
	for _, u := range pythonUnits(t, `class Outer:
    def __init__(self):
        def helper():
            pass

    class Inner:
        def __init__(self):
            pass
`) {
		if u.Kind == analysis.Function {
			names = append(names, u.Name)
		}
	}
	want := []string{"Outer.__init__", "Outer.Inner.__init__"}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Errorf("want function Units %v, got %v", want, names)
	}
}
