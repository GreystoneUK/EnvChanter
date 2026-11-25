package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
)

type syncOptions struct {
	envFile string
	force   bool
	quotes  bool
}

type syncDifference struct {
	key         string
	reference   string
	localValue  string
	localExists bool
	remoteValue string
}

func runSync(
	ctx context.Context,
	fetch func(context.Context, ParameterMap) (map[string]string, error),
	mapping ParameterMap,
	options syncOptions,
	input io.Reader,
	output io.Writer,
) error {
	local, err := readEnvFile(options.envFile)
	if err != nil {
		return fmt.Errorf("read .env file: %w", err)
	}

	remote, err := fetch(ctx, mapping)
	if err != nil {
		return fmt.Errorf("fetch remote values: %w", err)
	}

	keys := make([]string, 0, len(mapping))
	for key := range mapping {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var differences []syncDifference
	for _, key := range keys {
		localValue, localExists := local[key]
		remoteValue, remoteExists := remote[key]
		if !remoteExists {
			if localExists {
				fmt.Fprintf(output, "Warning: remote value not found for %s; keeping local value.\n", key)
			} else {
				fmt.Fprintf(output, "Warning: remote value not found for %s; skipping.\n", key)
			}
			continue
		}

		if !localExists || localValue != remoteValue {
			differences = append(differences, syncDifference{
				key:         key,
				reference:   mapping[key],
				localValue:  localValue,
				localExists: localExists,
				remoteValue: remoteValue,
			})
		}
	}

	if len(differences) == 0 {
		fmt.Fprintln(output, "No updates needed.")
		return nil
	}

	fmt.Fprintf(output, "\nFound %d value(s) with differences:\n\n", len(differences))
	for i, diff := range differences {
		fmt.Fprintf(output, "%d. %s\n", i+1, diff.key)
		if diff.localExists {
			fmt.Fprintf(output, "   Local:  %s\n", diff.localValue)
		} else {
			fmt.Fprintln(output, "   Local:  (not set)")
		}
		fmt.Fprintf(output, "   Remote: %s\n   Reference: %s\n\n", diff.remoteValue, diff.reference)
	}

	selected := differences
	if options.force {
		fmt.Fprintf(output, "Force mode enabled. Updating all %d value(s)...\n", len(selected))
	} else {
		selected, err = selectSyncUpdates(input, output, differences)
		if err != nil {
			return err
		}
		if selected == nil {
			fmt.Fprintln(output, "Sync cancelled. No updates applied.")
			return nil
		}
	}

	if len(selected) == 0 {
		fmt.Fprintln(output, "No values selected for update.")
		return nil
	}

	for _, diff := range selected {
		local[diff.key] = diff.remoteValue
	}
	if err := writeEnvFile(options.envFile, local, options.quotes); err != nil {
		return fmt.Errorf("write updated .env file: %w", err)
	}

	fmt.Fprintf(output, "\n✓ Successfully updated %s with %d remote value(s)\n", options.envFile, len(selected))
	return nil
}

func selectSyncUpdates(input io.Reader, output io.Writer, differences []syncDifference) ([]syncDifference, error) {
	scanner := bufio.NewScanner(input)
	selected := make([]syncDifference, 0, len(differences))

	for i, diff := range differences {
		for {
			fmt.Fprintf(output, "Update %s (%d/%d)? [y]es/[n]o/[a]ll/[c]ancel: ", diff.key, i+1, len(differences))
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					return nil, fmt.Errorf("read sync selection: %w", err)
				}
				return nil, fmt.Errorf("input closed before sync selections were complete: %w", io.EOF)
			}

			switch strings.ToLower(strings.TrimSpace(scanner.Text())) {
			case "y", "yes":
				selected = append(selected, diff)
			case "n", "no":
			case "a", "all":
				return append(selected, differences[i:]...), nil
			case "c", "cancel":
				return nil, nil
			default:
				fmt.Fprintln(output, "Invalid input. Please enter y(es), n(o), a(ll), or c(ancel).")
				continue
			}
			break
		}
	}

	return selected, nil
}
