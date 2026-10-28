package term

import "testing"

func TestClampSize(t *testing.T) {
	cases := []struct{ cols, rows, wantCols, wantRows int }{
		{80, 24, 80, 24},
		{0, 0, 1, 1},
		{-5, -1, 1, 1},
		{65535, 65535, 4096, 1024}, // total-cell cap shrinks rows
		{4097, 1, 4096, 1},
		{1, 4097, 1, 4096},
		{4096, 4096, 4096, 1024},
		{2048, 2048, 2048, 2048}, // exactly 1<<22 cells is allowed
	}
	for _, c := range cases {
		cols, rows := clampSize(c.cols, c.rows)
		if cols != c.wantCols || rows != c.wantRows {
			t.Errorf("clampSize(%d,%d) = %d,%d want %d,%d", c.cols, c.rows, cols, rows, c.wantCols, c.wantRows)
		}
		if cols*rows > 1<<22 {
			t.Errorf("clampSize(%d,%d) = %d cells, over the cap", c.cols, c.rows, cols*rows)
		}
	}
}
