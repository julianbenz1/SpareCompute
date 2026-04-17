package scheduler

import (
	"errors"
	"sort"

	"github.com/julianbenz1/SpareCompute/internal/common"
)

var ErrNoSuitableNode = errors.New("no suitable node available")

func SelectNode(nodes []common.Node, d common.Deployment, excludeNodeID string) (common.Node, error) {
	candidates := make([]common.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.ID == excludeNodeID {
			continue
		}
		if !CanHost(n, d) {
			continue
		}
		candidates = append(candidates, n)
	}
	if len(candidates) == 0 {
		return common.Node{}, ErrNoSuitableNode
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := score(candidates[i], d)
		right := score(candidates[j], d)
		if left == right {
			return candidates[i].ID < candidates[j].ID
		}
		return left > right
	})
	return candidates[0], nil
}

func CanHost(n common.Node, d common.Deployment) bool {
	if n.Status != common.NodeOnline || n.MaintenanceMode {
		return false
	}
	if n.ShareableCPU < d.CPULimit {
		return false
	}
	if n.ShareableRAMMB < d.RAMLimitMB {
		return false
	}
	if n.ShareableDiskMB < d.DiskLimitMB {
		return false
	}
	return true
}

func NeedsMigration(current common.Node, d common.Deployment) bool {
	return !CanHost(current, d)
}

func score(n common.Node, d common.Deployment) int64 {
	cpuSlack := int64(n.ShareableCPU - d.CPULimit)
	ramSlack := n.ShareableRAMMB - d.RAMLimitMB
	diskSlack := n.ShareableDiskMB - d.DiskLimitMB
	return cpuSlack + ramSlack + diskSlack
}
