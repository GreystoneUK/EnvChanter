package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSync(t *testing.T) {
	original := "# Keep this when sync does not write\nA=old-a\nB=old-b\nEMPTY=\nKEEP=local-only\nUNMAPPED=keep\n"
	changed := map[string]string{
		"A": "new-a", "B": "new-b", "EMPTY": "new-empty", "NEW": "new-value", "UNMAPPED": "ignore",
	}
	remoteFailure := errors.New("remote unavailable")

	for _, tt := range []struct {
		name       string
		remote     map[string]string
		fetchErr   error
		input      string
		force      bool
		quotes     bool
		wantFile   string
		wantOutput []string
		noOutput   []string
		wantErr    error
	}{
		{
			name:     "select updates after retrying invalid input",
			remote:   changed,
			input:    "invalid\ny\nn\nn\ny\n",
			wantFile: "A=new-a\nB=old-b\nEMPTY=\nKEEP=local-only\nNEW=new-value\nUNMAPPED=keep\n",
			wantOutput: []string{
				"Warning: remote value not found for ABSENT; skipping.",
				"Warning: remote value not found for KEEP; keeping local value.",
				"1. A\n   Local:  old-a\n   Remote: new-a",
				"3. EMPTY\n   Local:  \n   Remote: new-empty",
				"4. NEW\n   Local:  (not set)\n   Remote: new-value",
				"Invalid input.",
				"Update NEW (4/4)?",
				"with 2 remote value(s)",
			},
			noOutput: []string{"Update ABSENT", "Update KEEP"},
		},
		{
			name:       "force preserves missing and unmapped values",
			remote:     changed,
			force:      true,
			quotes:     true,
			wantFile:   "A=\"new-a\"\nB=\"new-b\"\nEMPTY=\"new-empty\"\nKEEP=\"local-only\"\nNEW=\"new-value\"\nUNMAPPED=\"keep\"\n",
			wantOutput: []string{"keeping local value.", "Local:  old-a", "Remote: new-a", "Force mode enabled."},
			noOutput:   []string{"Update A ("},
		},
		{
			name:       "all applies only remaining updates",
			remote:     changed,
			input:      "n\na",
			wantFile:   "A=old-a\nB=new-b\nEMPTY=new-empty\nKEEP=local-only\nNEW=new-value\nUNMAPPED=keep\n",
			wantOutput: []string{"with 3 remote value(s)"},
			noOutput:   []string{"Update EMPTY ("},
		},
		{
			name:       "cancel discards earlier selection",
			remote:     changed,
			input:      "y\nc\n",
			wantFile:   original,
			wantOutput: []string{"Sync cancelled. No updates applied."},
			noOutput:   []string{"Successfully updated"},
		},
		{
			name:     "closed input discards earlier selection",
			remote:   changed,
			input:    "y\n",
			wantFile: original,
			wantErr:  io.EOF,
			noOutput: []string{"Successfully updated"},
		},
		{
			name:       "declining all avoids rewriting file",
			remote:     changed,
			input:      "n\nn\nn\nn\n",
			wantFile:   original,
			wantOutput: []string{"No values selected for update."},
		},
		{
			name:       "no differences avoids prompting or rewriting file",
			remote:     map[string]string{"A": "old-a", "B": "old-b", "EMPTY": "", "KEEP": "local-only"},
			wantFile:   original,
			wantOutput: []string{"No updates needed."},
			noOutput:   []string{"Update A ("},
		},
		{
			name:     "remote failure leaves file untouched",
			remote:   changed,
			fetchErr: remoteFailure,
			wantFile: original,
			wantErr:  remoteFailure,
			noOutput: []string{"Update A (", "Successfully updated"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			envFile := filepath.Join(t.TempDir(), ".env")
			if err := os.WriteFile(envFile, []byte(original), 0600); err != nil {
				t.Fatal(err)
			}

			mapping := ParameterMap{
				"A": "/a", "ABSENT": "/absent", "B": "/b", "EMPTY": "/empty", "KEEP": "/keep", "NEW": "/new",
			}
			fetch := func(_ context.Context, got ParameterMap) (map[string]string, error) {
				if !maps.Equal(got, mapping) {
					t.Fatalf("provider mapping = %v, want %v", got, mapping)
				}
				return tt.remote, tt.fetchErr
			}

			var output bytes.Buffer
			var input io.Reader
			if !tt.force {
				input = strings.NewReader(tt.input)
			}
			options := syncOptions{envFile: envFile, force: tt.force, quotes: tt.quotes}
			err := runSync(context.Background(), fetch, mapping, options, input, &output)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("runSync error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == io.EOF && !strings.Contains(err.Error(), "input closed") {
				t.Errorf("runSync returned an unclear closed input error: %v", err)
			}

			got, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.wantFile {
				t.Errorf(".env = %q, want %q", got, tt.wantFile)
			}
			for _, want := range tt.wantOutput {
				if !strings.Contains(output.String(), want) {
					t.Errorf("output missing %q: %q", want, output.String())
				}
			}
			for _, unwanted := range tt.noOutput {
				if strings.Contains(output.String(), unwanted) {
					t.Errorf("output contains %q: %q", unwanted, output.String())
				}
			}
		})
	}
}
