package scheduler

import (
	"testing"

	"github.com/julianbenz1/SpareCompute/internal/common"
)

func TestSelectNode(t *testing.T) {
	nodes := []common.Node{
		{ID: "a", Status: common.NodeOnline, ShareableCPU: 20, ShareableRAMMB: 1024, ShareableDiskMB: 1000},
		{ID: "b", Status: common.NodeOnline, ShareableCPU: 70, ShareableRAMMB: 4096, ShareableDiskMB: 5000},
	}
	dep := common.Deployment{CPULimit: 30, RAMLimitMB: 512, DiskLimitMB: 100}
	got, err := SelectNode(nodes, dep, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "b" {
		t.Fatalf("expected node b, got %s", got.ID)
	}
}

func TestNeedsMigration(t *testing.T) {
	node := common.Node{ID: "a", Status: common.NodeOnline, ShareableCPU: 20, ShareableRAMMB: 200, ShareableDiskMB: 100}
	dep := common.Deployment{CPULimit: 30, RAMLimitMB: 100, DiskLimitMB: 10}
	if !NeedsMigration(node, dep) {
		t.Fatal("expected migration to be needed")
	}
}

