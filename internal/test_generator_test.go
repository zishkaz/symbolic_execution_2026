package internal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateTestFile проверяет генерацию тестового файла для небольшого
// исходного файла: файл создаётся рядом с исходным, компилируется и тесты проходят
func TestGenerateTestFile(t *testing.T) {
	tmpDir := t.TempDir()

	source := `package main

func add(a, b int) int {
	if a > b {
		return a + b
	}
	return a - b
}

func isPositive(x int) bool {
	return x > 0
}

func unsupported(f float64) float64 {
	return f + 1.0
}
`
	sourceFile := filepath.Join(tmpDir, "functions.go")
	if err := os.WriteFile(sourceFile, []byte(source), 0644); err != nil {
		t.Fatalf("не удалось записать исходный файл: %v", err)
	}
	// Минимальный go.mod, чтобы сгенерированный тест можно было скомпилировать
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module tmpgen\n\ngo 1.23\n"), 0644); err != nil {
		t.Fatalf("не удалось записать go.mod: %v", err)
	}

	testFile := GenerateTestFile(sourceFile)

	expectedPath := filepath.Join(tmpDir, "functions_test.go")
	if testFile != expectedPath {
		t.Errorf("Ожидался путь '%s', получен '%s'", expectedPath, testFile)
	}

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("тестовый файл не создан: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "package main") || !strings.Contains(text, "import \"testing\"") {
		t.Errorf("Сгенерированный файл не содержит package main и import \"testing\":\n%s", text)
	}
	if !strings.Contains(text, "func TestAdd(") || !strings.Contains(text, "func TestIsPositive(") {
		t.Errorf("Сгенерированный файл не содержит тесты для add и isPositive:\n%s", text)
	}
	// Функция с float64 не поддерживается анализатором и должна быть пропущена
	if strings.Contains(text, "func TestUnsupported(") {
		t.Errorf("Для неподдерживаемой функции unsupported не должно генерироваться теста:\n%s", text)
	}

	// Сгенерированный файл должен компилироваться, а тесты — проходить
	cmd := exec.Command("go", "test", ".")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test на сгенерированном файле завершился с ошибкой: %v\n%s", err, output)
	}
}
