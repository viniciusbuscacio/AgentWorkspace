package browser

import (
	"context"
	"testing"
)

func TestCloseTabsRequiresAMatcher(t *testing.T) {
	if _, err := closeTabs(context.Background(), 0, map[string]any{}); err == nil {
		t.Fatal("close_tab with no tab/url/title must error")
	}
}
