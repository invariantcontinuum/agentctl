package catalog

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/invariantcontinuum/agentctl/internal/agent"
)

type Image struct {
	Ref         string
	Role        string
	Description string
	Taxonomy    map[string]string
}

type Catalog struct {
	images []Image
}

func DefaultCatalog() Catalog {
	return Catalog{images: []Image{
		newRoleImage("planner:latest", "planner", "Decomposes goals, sequences work, and chooses next actions."),
		newRoleImage("researcher:latest", "researcher", "Retrieves context and summarizes findings from knowledge sources."),
		newRoleImage("coder:latest", "coder", "Edits code, runs checks, and reports implementation results."),
		newRoleImage("reviewer:latest", "reviewer", "Reviews changes for correctness, risks, and missing tests."),
		newRoleImage("executor:latest", "executor", "Runs bounded operational tasks and tool-driven workflows."),
		newRoleImage("coordinator:latest", "coordinator", "Coordinates multi-agent delegation and completion criteria."),
	}}
}

func newRoleImage(ref string, role string, description string) Image {
	return Image{
		Ref:         ref,
		Role:        role,
		Description: description,
		Taxonomy: map[string]string{
			"knowledge":      "rag,vector,graph",
			"action":         "tools,mcp,function-calling",
			"persistence":    "memory,history,summaries,episodic",
			"control":        "planning,loop,evaluation",
			"specialization": role,
		},
	}
}

func (c Catalog) List() []Image {
	images := append([]Image{}, c.images...)
	sort.Slice(images, func(left, right int) bool {
		return images[left].Ref < images[right].Ref
	})
	return images
}

func (c Catalog) Find(ref string) (Image, bool) {
	for _, image := range c.images {
		if image.Ref == ref {
			return image, true
		}
	}
	return Image{}, false
}

func (c Catalog) MustConfig(ref string) (agent.Config, error) {
	image, ok := c.Find(ref)
	if !ok {
		return agent.Config{}, fmt.Errorf("unknown agent image %q", ref)
	}

	return agent.Config{
		Image: ref,
		Name:  image.Role,
		Type:  image.Role,
		Model: agent.Model{
			Provider: "vllm",
			Name:     "local",
			Endpoint: "http://localhost:8000/v1",
			Auth:     "none",
		},
		Skills: []agent.Skill{
			{Source: "builtin://skills/" + image.Role},
		},
		Loop: agent.Loop{
			Strategy: "react",
			MaxSteps: 30,
		},
		Labels: map[string]string{
			"agentctl.taxonomy.knowledge":      image.Taxonomy["knowledge"],
			"agentctl.taxonomy.action":         image.Taxonomy["action"],
			"agentctl.taxonomy.persistence":    image.Taxonomy["persistence"],
			"agentctl.taxonomy.control":        image.Taxonomy["control"],
			"agentctl.taxonomy.specialization": image.Taxonomy["specialization"],
		},
		Env: map[string]string{
			"AGENTCTL_AGENT_IMAGE": ref,
			"AGENTCTL_AGENT_ROLE":  image.Role,
		},
		Exec: []string{
			"sh",
			"-c",
			fmt.Sprintf("echo %s started; trap 'exit 0' TERM INT; while true; do sleep 60; done", ref),
		},
	}, nil
}

func (c Catalog) WriteTable(writer io.Writer) error {
	if _, err := fmt.Fprintf(writer, "%-22s %-14s %s\n", "IMAGE", "ROLE", "DESCRIPTION"); err != nil {
		return err
	}
	for _, image := range c.List() {
		if _, err := fmt.Fprintf(writer, "%-22s %-14s %s\n", image.Ref, image.Role, image.Description); err != nil {
			return err
		}
	}
	return nil
}

func IsImageRef(value string) bool {
	return strings.Contains(value, ":")
}
