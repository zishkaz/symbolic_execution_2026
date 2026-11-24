package symbolic

import (
	"testing"
)

func TestSymbolicVariable(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	if x.Type() != IntType {
		t.Errorf("Expected IntType, got %s", x.Type())
	}
	if x.String() != "x" {
		t.Errorf("Expected 'x', got '%s'", x.String())
	}

	b := NewSymbolicVariable("flag", BoolType)
	if b.Type() != BoolType {
		t.Errorf("Expected BoolType, got %s", b.Type())
	}
}

func TestIntConstant(t *testing.T) {
	five := NewIntConstant(5)
	if five.Type() != IntType {
		t.Errorf("Expected IntType, got %s", five.Type())
	}
	if five.String() != "5" {
		t.Errorf("Expected '5', got '%s'", five.String())
	}
}

func TestBoolConstant(t *testing.T) {
	tr := NewBoolConstant(true)
	if tr.Type() != BoolType {
		t.Errorf("Expected BoolType, got %s", tr.Type())
	}
	if tr.String() != "true" {
		t.Errorf("Expected 'true', got '%s'", tr.String())
	}
}

func TestBinaryOperationArithmeticType(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)

	ops := []BinaryOperator{ADD, SUB, MUL, DIV, MOD}
	for _, op := range ops {
		expr := NewBinaryOperation(x, y, op)
		if expr.Type() != IntType {
			t.Errorf("Operation %s: expected IntType, got %s", op, expr.Type())
		}
	}
}

func TestBinaryOperationComparisonType(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)

	ops := []BinaryOperator{EQ, NE, LT, LE, GT, GE}
	for _, op := range ops {
		expr := NewBinaryOperation(x, y, op)
		if expr.Type() != BoolType {
			t.Errorf("Operation %s: expected BoolType, got %s", op, expr.Type())
		}
	}
}

func TestBinaryOperationString(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)
	five := NewIntConstant(5)

	sum := NewBinaryOperation(x, y, ADD)
	if sum.String() != "(x + y)" {
		t.Errorf("Expected '(x + y)', got '%s'", sum.String())
	}

	condition := NewBinaryOperation(sum, five, GT)
	if condition.String() != "((x + y) > 5)" {
		t.Errorf("Expected '((x + y) > 5)', got '%s'", condition.String())
	}
}

func TestBinaryOperationTypeMismatchPanics(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	flag := NewSymbolicVariable("flag", BoolType)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on type mismatch")
		}
	}()
	NewBinaryOperation(x, flag, ADD)
}

func TestLogicalOperationAndString(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)
	zero := NewIntConstant(0)
	ten := NewIntConstant(10)

	cond1 := NewBinaryOperation(x, zero, GT)
	cond2 := NewBinaryOperation(y, ten, LT)

	andExpr := NewLogicalOperation([]SymbolicExpression{cond1, cond2}, AND)
	expected := "((x > 0) && (y < 10))"
	if andExpr.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, andExpr.String())
	}
	if andExpr.Type() != BoolType {
		t.Errorf("Expected BoolType, got %s", andExpr.Type())
	}
}

func TestLogicalOperationOrString(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)
	zero := NewIntConstant(0)

	cond1 := NewBinaryOperation(x, zero, GT)
	cond2 := NewBinaryOperation(y, zero, GT)
	cond3 := NewBinaryOperation(x, NewIntConstant(100), LT)

	orExpr := NewLogicalOperation([]SymbolicExpression{cond1, cond2, cond3}, OR)
	expected := "((x > 0) || (y > 0) || (x < 100))"
	if orExpr.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, orExpr.String())
	}
}

func TestLogicalOperationNotString(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	zero := NewIntConstant(0)

	cond := NewBinaryOperation(x, zero, GT)
	notExpr := NewLogicalOperation([]SymbolicExpression{cond}, NOT)
	expected := "!(x > 0)"
	if notExpr.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, notExpr.String())
	}
}

func TestLogicalOperationImpliesString(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	zero := NewIntConstant(0)

	antecedent := NewBinaryOperation(x, zero, GT)
	consequent := NewBinaryOperation(x, NewIntConstant(10), LT)

	implies := NewLogicalOperation([]SymbolicExpression{antecedent, consequent}, IMPLIES)
	expected := "((x > 0) => (x < 10))"
	if implies.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, implies.String())
	}
}

func TestLogicalOperationArityPanics(t *testing.T) {
	x := NewSymbolicVariable("x", BoolType)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on NOT with 2 operands")
		}
	}()
	NewLogicalOperation([]SymbolicExpression{x, x}, NOT)
}

func TestLogicalOperationNonBoolPanics(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on non-bool operand")
		}
	}()
	NewLogicalOperation([]SymbolicExpression{x, y}, AND)
}

func TestUnaryOperation(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)

	neg := NewUnaryOperation(x, UMINUS)
	if neg.Type() != IntType {
		t.Errorf("Expected IntType, got %s", neg.Type())
	}
	if neg.String() != "-x" {
		t.Errorf("Expected '-x', got '%s'", neg.String())
	}

	flag := NewSymbolicVariable("flag", BoolType)
	notFlag := NewUnaryOperation(flag, LNOT)
	if notFlag.Type() != BoolType {
		t.Errorf("Expected BoolType, got %s", notFlag.Type())
	}
	if notFlag.String() != "!flag" {
		t.Errorf("Expected '!flag', got '%s'", notFlag.String())
	}
}

func TestUnaryOperationTypePanics(t *testing.T) {
	x := NewSymbolicVariable("x", IntType)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic on LNOT with int operand")
		}
	}()
	NewUnaryOperation(x, LNOT)
}

func TestRef(t *testing.T) {
	ref := NewRef(1, ArrayType)
	if ref.Type() != ArrayType {
		t.Errorf("Expected ArrayType, got %s", ref.Type())
	}
	expected := "ref#1(array)"
	if ref.String() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, ref.String())
	}
}

// countingVisitor считает количество посещённых узлов для проверки обхода
type countingVisitor struct {
	variables int
	constants int
	binaries  int
	logicals  int
	unaries   int
}

func (cv *countingVisitor) VisitVariable(expr *SymbolicVariable) interface{} {
	cv.variables++
	return nil
}

func (cv *countingVisitor) VisitIntConstant(expr *IntConstant) interface{} {
	cv.constants++
	return nil
}

func (cv *countingVisitor) VisitBoolConstant(expr *BoolConstant) interface{} {
	cv.constants++
	return nil
}

func (cv *countingVisitor) VisitBinaryOperation(expr *BinaryOperation) interface{} {
	cv.binaries++
	expr.Left.Accept(cv)
	expr.Right.Accept(cv)
	return nil
}

func (cv *countingVisitor) VisitLogicalOperation(expr *LogicalOperation) interface{} {
	cv.logicals++
	for _, operand := range expr.Operands {
		operand.Accept(cv)
	}
	return nil
}

func (cv *countingVisitor) VisitUnaryOperation(expr *UnaryOperation) interface{} {
	cv.unaries++
	expr.Operand.Accept(cv)
	return nil
}

func (cv *countingVisitor) VisitRef(expr *Ref) interface{} {
	return nil
}

func TestVisitorTraversal(t *testing.T) {
	// Выражение: (x + y) > 5
	x := NewSymbolicVariable("x", IntType)
	y := NewSymbolicVariable("y", IntType)
	five := NewIntConstant(5)
	sum := NewBinaryOperation(x, y, ADD)
	condition := NewBinaryOperation(sum, five, GT)

	cv := &countingVisitor{}
	condition.Accept(cv)

	if cv.variables != 2 {
		t.Errorf("Expected 2 variables, got %d", cv.variables)
	}
	if cv.constants != 1 {
		t.Errorf("Expected 1 constant, got %d", cv.constants)
	}
	if cv.binaries != 2 {
		t.Errorf("Expected 2 binary operations, got %d", cv.binaries)
	}
}
