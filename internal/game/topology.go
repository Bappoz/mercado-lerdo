package game

import (
	"github.com/Bappoz/software-engineering-simulator/internal/stack"
	"github.com/Bappoz/software-engineering-simulator/internal/topology"
)

// TopologyView is the player's system as the workspace describes it and as
// it runs, with what a deploy would change.
type TopologyView struct {
	Workspace      *topology.Topology `json:"workspace"`       // nil when the file is invalid
	WorkspaceError string             `json:"workspace_error"` // why it is invalid
	WorkspaceCost  float64            `json:"workspace_cost"`
	Deployed       *topology.Topology `json:"deployed"` // nil while the sandbox is down
	DeployedCost   float64            `json:"deployed_cost"`
	Pending        []stack.Change     `json:"pending"`
	Budget         float64            `json:"budget"` // 0: the scenario grades no cost
}

// Topology describes the workspace's topology against the running one.
func (g *Game) Topology() TopologyView {
	var v TopologyView
	for _, o := range g.Pack.Manifest.Objectives {
		if o.Measure == topology.CostMeasure && o.Below != nil {
			v.Budget = *o.Below
		}
	}
	ws, err := topology.Load(g.Session().Workspace)
	if err != nil {
		v.WorkspaceError = err.Error()
	} else {
		v.Workspace, v.WorkspaceCost = &ws, ws.MonthlyCost()
	}
	if st := g.running(); st != nil {
		dep := st.Topology()
		v.Deployed, v.DeployedCost = &dep, dep.MonthlyCost()
		if v.Workspace != nil {
			v.Pending, err = st.Plan(ws)
			if err != nil {
				v.WorkspaceError = err.Error()
			}
		}
	}
	return v
}

// Install adds an optional catalog component to the workspace's
// topology.yaml. Nothing runs until the next deploy.
func (g *Game) Install(k topology.Kind) error {
	if !g.HasSession() {
		return ErrNoSession
	}
	if err := topology.InstallFile(g.Session().Workspace, k); err != nil {
		return err
	}
	g.Event("component_installed", map[string]any{"kind": string(k)})
	return nil
}
