// Package symbolic определяет базовые типы символьных выражений
package symbolic

// ExpressionType представляет тип символьного выражения
type ExpressionType int

const (
	IntType ExpressionType = iota
	BoolType
	ArrayType
	StructType
	// Добавьте другие типы по необходимости
)

// String возвращает строковое представление типа
func (et ExpressionType) String() string {
	switch et {
	case IntType:
		return "int"
	case BoolType:
		return "bool"
	case ArrayType:
		return "array"
	case StructType:
		return "struct"
	default:
		return "unknown"
	}
}

// ArraySort описывает сорт символьного массива Array(Index -> Elem).
// Если элементы сами являются массивами (Elem == ArrayType),
// их сорт уточняется рекурсивно через Inner.
type ArraySort struct {
	Index ExpressionType
	Elem  ExpressionType
	Inner *ArraySort
}

// IntArraySort возвращает сорт массива целых чисел: Array(Int -> Int)
func IntArraySort() ArraySort {
	return ArraySort{Index: IntType, Elem: IntType}
}

// String возвращает строковое представление сорта массива
func (as ArraySort) String() string {
	if as.Elem == ArrayType && as.Inner != nil {
		return "array(" + as.Index.String() + " -> " + as.Inner.String() + ")"
	}
	return "array(" + as.Index.String() + " -> " + as.Elem.String() + ")"
}
