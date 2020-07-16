package testconsole

import (
	"strings"
	"testing"
)

func TestConsoleSplitString(t *testing.T) {
	str := `LocalRun {"senderId":1001,"targetId":1002,"content":"XXX BBB"}`

	strarr := strings.Split(str, " ")
	t.Logf("Split.Size: %d, %v", len(strarr), strarr)

	strarr = strings.SplitN(str, " ", 2)
	t.Logf("SplitN.Size: %d, %v", len(strarr), strarr)

	strarr = strings.SplitAfter(str, " ")
	t.Logf("SplitAfter.Size: %d, %v", len(strarr), strarr)

	strarr = strings.SplitAfterN(str, " ", 2)
	t.Logf("SplitAfterN.Size: %d, %v", len(strarr), strarr)
}
