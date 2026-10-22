package contracts

import (
	"testing"
)

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	if reg == nil {
		t.Fatal("DefaultRegistry returned nil")
	}

	docID := DocID("MessagingThemeQuery")
	if docID != DefaultDocIDMessagingThemeQuery {
		t.Errorf("expected %s, got %s", DefaultDocIDMessagingThemeQuery, docID)
	}

	url := GraphQLURL()
	if url != DefaultGraphQLURL {
		t.Errorf("expected %s, got %s", DefaultGraphQLURL, url)
	}

	op, err := GetOperation("ThreadListQuery")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if op.DocID != DefaultDocIDThreadList {
		t.Errorf("expected %s, got %s", DefaultDocIDThreadList, op.DocID)
	}
}

func TestRegistry_Override(t *testing.T) {
	t.Setenv("FBCHAT_DOCID_MESSAGINGTHEMEQUERY", "999888777")
	docID := DocID("MessagingThemeQuery")
	if docID != "999888777" {
		t.Errorf("expected overridden docID 999888777, got %s", docID)
	}

	t.Setenv("FBCHAT_URL_GRAPHQL", "https://custom.facebook.com/api/graphql/")
	url := GraphQLURL()
	if url != "https://custom.facebook.com/api/graphql/" {
		t.Errorf("expected overridden url, got %s", url)
	}
}
