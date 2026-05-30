package cmd

import "testing"

// TestKeySuffixerFlagBinding guards against the regression where --key-suffixer
// was bound to keyPrefixerOptions.
func TestKeySuffixerFlagBinding(t *testing.T) {
	prevPrefix, prevSuffix := keyPrefixerOptions, keySuffixerOptions
	keyPrefixerOptions, keySuffixerOptions = "", ""

	t.Cleanup(func() {
		keyPrefixerOptions, keySuffixerOptions = prevPrefix, prevSuffix
		_ = loadCmd.PersistentFlags().Set("key-suffixer", prevSuffix)
		_ = loadCmd.PersistentFlags().Set("key-prefixer", prevPrefix)
	})

	f := loadCmd.PersistentFlags().Lookup("key-suffixer")
	if f == nil {
		t.Fatal("key-suffixer flag is not registered")
	}

	if err := f.Value.Set("__SUFFIX__"); err != nil {
		t.Fatal(err)
	}

	if keySuffixerOptions != "__SUFFIX__" {
		t.Fatalf("--key-suffixer should set keySuffixerOptions, got %q", keySuffixerOptions)
	}

	if keyPrefixerOptions != "" {
		t.Fatalf("--key-suffixer must not touch keyPrefixerOptions, got %q", keyPrefixerOptions)
	}
}

// TestShouldUseElasticsearch guards against the strings.ContainsAny misuse: only
// an outputs list that actually contains the "elasticsearch" token should match.
func TestShouldUseElasticsearch(t *testing.T) {
	matches := []string{"elasticsearch", "stdout,elasticsearch", "elasticsearch,file"}
	for _, in := range matches {
		if !shouldUseElasticsearch(in) {
			t.Errorf("shouldUseElasticsearch(%q) = false, want true", in)
		}
	}

	nonMatches := []string{"", "stdout", "stderr", "file", "console", "stdout,file"}
	for _, in := range nonMatches {
		if shouldUseElasticsearch(in) {
			t.Errorf("shouldUseElasticsearch(%q) = true, want false", in)
		}
	}
}
