package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/app"
	"github.com/quantum-6/skillvault/internal/domain"
)

func runGraph(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseGraphFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	result, err := svc.entryRefSvc.GetEntryGraph(ctx, app.GetGraphInput{
		EntryID:   flags.EntryID,
		Direction: flags.Direction,
		MaxDepth:  flags.Depth,
	})
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	switch flags.Format {
	case "json":
		out := map[string]interface{}{
			"root_entry": flags.EntryID,
			"direction":  flags.Direction,
			"depth":      flags.Depth,
			"node_count": len(result.Nodes),
			"edge_count": len(result.Edges),
			"nodes":      result.Nodes,
			"edges":      result.Edges,
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		fmt.Println(string(data))

	case "dot":
		fmt.Println("digraph G {")
		fmt.Printf("  // Root: %s, %d nodes, %d edges\n", flags.EntryID, len(result.Nodes), len(result.Edges))
		for _, n := range result.Nodes {
			fmt.Printf("  %q;\n", n.EntryID)
		}
		for _, e := range result.Edges {
			label := e.Label
			if label == "" {
				label = string(e.RelationType)
			}
			fmt.Printf("  %q -> %q [label=%q];\n", e.FromEntryID, e.ToEntryID, label)
		}
		fmt.Println("}")

	default: // mermaid
		fmt.Println("graph TD")
		fmt.Printf("  %% Root: %s, depth %d, direction: %s\n", flags.EntryID, flags.Depth, flags.Direction)
		for _, e := range result.Edges {
			label := e.Label
			if label == "" {
				label = string(e.RelationType)
			}
			fmt.Printf("  %s -->|%s| %s\n", e.FromEntryID, label, e.ToEntryID)
		}
		if len(result.Edges) == 0 {
			fmt.Printf("  %s[\"No connections\"]\n", flags.EntryID)
		}
	}
}

func runEntryHistory(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseEntryHistoryFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	versions, err := svc.entryVersionSvc.ListVersions(ctx, flags.ID)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if len(versions) == 0 {
		fmt.Printf("No version history for entry %s.\n", flags.ID)
		return
	}

	fmt.Printf("Version history for entry %s:\n", flags.ID)
	fmt.Print(FormatTable(versions,
		[]string{"#", "Title", "Saved At"},
		func(v domain.EntryVersion) []string {
			return []string{
				fmt.Sprintf("%d", v.VersionNumber),
				v.Title,
				v.SavedAt,
			}
		}))
}

func runEntryRestore(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseEntryRestoreFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	if err := svc.entryVersionSvc.RestoreVersion(ctx, flags.ID, flags.Version); err != nil {
		PrintError(err)
		os.Exit(1)
	}

	fmt.Printf("Restored entry %s to version %d.\n", flags.ID, flags.Version)
}

func runEntryRef(ctx context.Context, svc *Services, args []string) {
	if len(args) < 4 {
		PrintError(fmt.Errorf("usage: skillvault entry ref <subcommand> [args...]"))
		os.Exit(1)
	}
	sub := args[3]
	switch sub {
	case "add":
		if len(args) < 7 {
			PrintError(fmt.Errorf("usage: skillvault entry ref add <source_id> <target_id> <ref_type> [--label <txt>]"))
			os.Exit(1)
		}
		sourceID := args[4]
		targetID := args[5]
		refType := args[6]
		label := ""
		for i, a := range args[7:] {
			if a == "--label" && i+1 < len(args[7:]) {
				label = args[7+i+1]
			}
		}
		link, err := svc.entryRefSvc.SaveRef(ctx, app.AddRefInput{
			SourceID: sourceID,
			TargetID: targetID,
			RefType:  refType,
			Label:    label,
		})
		if err != nil {
			PrintError(err)
			os.Exit(1)
		}
		fmt.Printf("Saved ref: %s --[%s]--> %s\n", link.FromEntryID, link.RelationType, link.ToEntryID)
		if link.Label != "" {
			fmt.Printf("  Label: %s\n", link.Label)
		}

	case "list":
		var srcPtr, tgtPtr, typePtr *string
		includeArchived := false
		for i, a := range args[4:] {
			v := ""
			if i+1 < len(args[4:]) {
				v = args[4+i+1]
			}
			switch a {
			case "--source":
				srcPtr = &v
			case "--target":
				tgtPtr = &v
			case "--type":
				typePtr = &v
			case "--include-archived":
				includeArchived = true
			}
		}
		links, err := svc.entryRefSvc.ListRefs(ctx, app.ListRefsInput{
			SourceID:        srcPtr,
			TargetID:        tgtPtr,
			RefType:         typePtr,
			IncludeArchived: includeArchived,
		})
		if err != nil {
			PrintError(err)
			os.Exit(1)
		}
		if len(links) == 0 {
			fmt.Println("No refs found.")
			return
		}
		fmt.Printf("Found %d ref(s):\n", len(links))
		for _, l := range links {
			la := ""
			if l.Label != "" {
				la = fmt.Sprintf(" (%s)", l.Label)
			}
			fmt.Printf("  %s --[%s%s]--> %s\n", l.FromEntryID, l.RelationType, la, l.ToEntryID)
		}

	case "remove":
		if len(args) < 7 {
			PrintError(fmt.Errorf("usage: skillvault entry ref remove <source_id> <target_id> <ref_type>"))
			os.Exit(1)
		}
		sourceID := args[4]
		targetID := args[5]
		refType := args[6]
		if err := svc.entryRefSvc.RemoveRef(ctx, sourceID, targetID, refType); err != nil {
			PrintError(err)
			os.Exit(1)
		}
		fmt.Printf("Removed ref: %s --[%s]--> %s\n", sourceID, refType, targetID)

	default:
		PrintError(fmt.Errorf("unknown entry ref subcommand: %s", sub))
		os.Exit(1)
	}
}
