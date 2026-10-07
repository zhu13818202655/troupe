package api

import (
	"github.com/zhu13818202655/troupe/pkg/sandbox"
	"github.com/zhu13818202655/troupe/pkg/sandbox/netpolicy"
)

func init() {
	// Wire NodeTool -> netpolicy PreSeed without an import cycle.
	// This is API-wide: chat, flows, app sandboxes, and fleet all share NodeTool.
	sandbox.NetworkPolicyPreSeeder = netpolicy.EnsurePreSeedFromContext
}
