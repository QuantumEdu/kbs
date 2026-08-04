package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/quantum-6/skillvault/internal/vector"
)

func runCompareEntries(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseCompareEntriesFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	diff, err := svc.compareSvc.CompareEntries(ctx, flags.ID1, flags.ID2)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	fmt.Print(diff)
}

func runSetupVectors(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseSetupVectorsFlags(args)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	gv, err := vector.LoadGlove(flags.Path)
	if err != nil {
		PrintError(fmt.Errorf("load GloVe vectors: %w", err))
		os.Exit(1)
	}

	fmt.Printf("Loaded %d word vectors (%d dimensions) from %s\n", gv.Len(), gv.Dims(), flags.Path)

	// Validate with a sample embedding.
	emb, err := vector.Embed("test query", gv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: test embedding failed: %v\n", err)
	} else if emb == nil {
		fmt.Fprintf(os.Stderr, "warning: test embedding produced no tokens\n")
	} else {
		fmt.Println("Test embedding OK.")
	}

	fmt.Println("\nTo enable vector search for future commands, set the environment variable:")
	fmt.Printf("  export SKILLVAULT_GLOVE_PATH=%s\n", flags.Path)
	fmt.Println("Then run: skillvault reindex-embeddings")
}

func runReindexEmbeddings(ctx context.Context, svc *Services, args []string) {
	flags, err := ParseReindexEmbeddingsFlags(args)
	_ = flags
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	count, err := svc.compareSvc.ReindexAll(ctx)
	if err != nil {
		PrintError(err)
		os.Exit(1)
	}

	fmt.Printf("Reindex complete: %d entries embedded.\n", count)
}
