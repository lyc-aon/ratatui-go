package autocomplete

import (
	"encoding/json"
	"strings"

	"github.com/lyc-aon/ratatui-go/ompui/model"
)

// rpcInputHint is the JSON shape of model.AvailableCommand.Input.
type rpcInputHint struct {
	Hint string `json:"hint"`
}

// rpcSubcommand is the JSON shape of model.AvailableCommand.Subcommands entries.
type rpcSubcommand struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Usage       string `json:"usage,omitempty"`
}

func slashFromModel(c model.AvailableCommand) SlashCommand {
	name := strings.TrimSpace(strings.TrimPrefix(c.Name, "/"))
	aliases := make([]string, 0, len(c.Aliases))
	for _, a := range c.Aliases {
		a = strings.TrimSpace(strings.TrimPrefix(a, "/"))
		if a != "" && a != name {
			aliases = append(aliases, a)
		}
	}
	sc := SlashCommand{
		Name:        name,
		Aliases:     aliases,
		Description: c.Description,
	}
	if hint := parseInputHint(c.Input); hint != "" {
		sc.ArgumentHint = hint
	}
	subs := parseSubcommands(c.Subcommands)
	if len(subs) > 0 {
		sc.ArgumentCompletions = buildArgumentCompletions(subs)
		sc.InlineHint = buildSubcommandInlineHint(subs)
	} else if sc.ArgumentHint != "" {
		hint := sc.ArgumentHint
		sc.InlineHint = buildStaticInlineHint(hint)
	}
	return sc
}

func parseInputHint(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var h rpcInputHint
	if err := json.Unmarshal(raw, &h); err != nil {
		return ""
	}
	return strings.TrimSpace(h.Hint)
}

func parseSubcommands(raw json.RawMessage) []Subcommand {
	if len(raw) == 0 {
		return nil
	}
	var subs []rpcSubcommand
	if err := json.Unmarshal(raw, &subs); err != nil {
		return nil
	}
	out := make([]Subcommand, 0, len(subs))
	for _, s := range subs {
		name := strings.TrimSpace(s.Name)
		if name == "" {
			continue
		}
		out = append(out, Subcommand{
			Name:        name,
			Description: s.Description,
			Usage:       s.Usage,
		})
	}
	return out
}

// buildArgumentCompletions mirrors OMP buildArgumentCompletions.
func buildArgumentCompletions(subcommands []Subcommand) func(string) []Item {
	return func(argumentPrefix string) []Item {
		if strings.Contains(argumentPrefix, " ") {
			return nil // past the subcommand token
		}
		lower := strings.ToLower(argumentPrefix)
		var matches []Item
		for _, s := range subcommands {
			if !strings.HasPrefix(strings.ToLower(s.Name), lower) {
				continue
			}
			matches = append(matches, Item{
				Value:       s.Name + " ",
				Label:       s.Name,
				Description: s.Description,
				Hint:        s.Usage,
			})
		}
		if len(matches) == 0 {
			return nil
		}
		return matches
	}
}

// buildSubcommandInlineHint mirrors OMP buildSubcommandInlineHint.
func buildSubcommandInlineHint(subcommands []Subcommand) func(string) string {
	return func(argumentText string) string {
		trimmed := strings.TrimLeft(argumentText, " \t")
		spaceIndex := strings.IndexByte(trimmed, ' ')

		if spaceIndex == -1 {
			prefix := strings.ToLower(trimmed)
			if prefix == "" {
				return ""
			}
			for _, s := range subcommands {
				if strings.HasPrefix(strings.ToLower(s.Name), prefix) {
					remaining := s.Name[len(prefix):]
					// Preserve original case suffix from the matched name.
					// prefix is lower; use rune-safe slice via byte len of lower prefix
					// which equals name prefix length for ASCII command names.
					if len(prefix) <= len(s.Name) && strings.EqualFold(s.Name[:len(prefix)], prefix) {
						remaining = s.Name[len(prefix):]
					}
					if s.Usage != "" {
						return remaining + " " + s.Usage
					}
					return remaining
				}
			}
			return ""
		}

		subName := strings.ToLower(trimmed[:spaceIndex])
		afterSub := trimmed[spaceIndex+1:]
		var sub *Subcommand
		for i := range subcommands {
			if strings.EqualFold(subcommands[i].Name, subName) {
				sub = &subcommands[i]
				break
			}
		}
		if sub == nil || sub.Usage == "" {
			return ""
		}
		if afterSub != "" {
			usageParts := strings.Fields(sub.Usage)
			inputParts := strings.Fields(strings.TrimSpace(afterSub))
			if len(inputParts) >= len(usageParts) {
				return ""
			}
			return strings.Join(usageParts[len(inputParts):], " ")
		}
		return sub.Usage
	}
}

func buildStaticInlineHint(hint string) func(string) string {
	return func(argumentText string) string {
		if strings.TrimSpace(argumentText) == "" {
			return hint
		}
		return ""
	}
}

type scoredItem struct {
	item  Item
	score int
	order int // stable registry order
}

func commandName(cmd SlashCommand) string {
	return cmd.Name
}

func commandAliases(cmd SlashCommand) []string {
	out := make([]string, 0, len(cmd.Aliases))
	for _, a := range cmd.Aliases {
		if a != "" {
			out = append(out, a)
		}
	}
	return out
}

func commandMatchesNameOrAlias(cmd SlashCommand, commandName string) bool {
	if cmd.Name == commandName {
		return true
	}
	for _, a := range cmd.Aliases {
		if a == commandName {
			return true
		}
	}
	return false
}

func staticCommandDescription(cmd SlashCommand) string {
	return cmd.Description
}

func autocompleteCommandDescription(cmd SlashCommand) string {
	if cmd.GetAutocompleteDescription != nil {
		if d := cmd.GetAutocompleteDescription(); d != "" {
			return d
		}
	}
	return cmd.Description
}

// buildSlashCommandCompletions mirrors OMP buildSlashCommandCompletions.
// Stable sort: higher score first; equal scores keep registry order.
func buildSlashCommandCompletions(commands []SlashCommand, lowerPrefix string) []Item {
	var hits []scoredItem
	order := 0
	for _, cmd := range commands {
		name := commandName(cmd)
		if name == "" {
			continue
		}
		hint := cmd.ArgumentHint
		staticDesc := staticCommandDescription(cmd)
		var fullDescMemo string
		fullDescComputed := false
		resolveFullDesc := func() string {
			if !fullDescComputed {
				displayDesc := autocompleteCommandDescription(cmd)
				if hint != "" {
					if displayDesc != "" {
						fullDescMemo = hint + " - " + displayDesc
					} else {
						fullDescMemo = hint
					}
				} else {
					fullDescMemo = displayDesc
				}
				fullDescComputed = true
			}
			return fullDescMemo
		}

		isSkillCommand := strings.HasPrefix(name, "skill:")
		nameScore := 0
		if lowerPrefix == "" && isSkillCommand {
			nameScore = 950
		} else {
			nameScore = scoreCommandTextMatch(lowerPrefix, strings.ToLower(name))
		}
		descScore := 0
		lowerDesc := strings.ToLower(staticDesc)
		if lowerDesc != "" && fuzzyMatch(lowerPrefix, lowerDesc) {
			descScore = int(float64(fuzzyScore(lowerPrefix, lowerDesc)) * 0.5)
		}
		primaryScore := nameScore
		if descScore > primaryScore {
			primaryScore = descScore
		}
		if primaryScore > 0 {
			fullDesc := resolveFullDesc()
			it := Item{
				Value: name,
				Label: name,
			}
			if fullDesc != "" {
				it.Description = fullDesc
			}
			hits = append(hits, scoredItem{item: it, score: primaryScore, order: order})
			order++
		}

		if lowerPrefix != "" {
			for _, alias := range commandAliases(cmd) {
				if alias == name {
					continue
				}
				aliasScore := scoreCommandTextMatch(lowerPrefix, strings.ToLower(alias))
				if aliasScore == 0 {
					continue
				}
				fullDesc := resolveFullDesc()
				it := Item{
					Value: alias,
					Label: alias,
				}
				if fullDesc != "" {
					it.Description = fullDesc
				}
				hits = append(hits, scoredItem{item: it, score: aliasScore, order: order})
				order++
			}
		}
	}

	// Stable sort by score desc, then original order.
	for i := 1; i < len(hits); i++ {
		j := i
		for j > 0 {
			better := hits[j].score > hits[j-1].score ||
				(hits[j].score == hits[j-1].score && hits[j].order < hits[j-1].order)
			if !better {
				break
			}
			hits[j], hits[j-1] = hits[j-1], hits[j]
			j--
		}
	}

	out := make([]Item, len(hits))
	for i, h := range hits {
		out[i] = h.item
	}
	return out
}

func findCommand(commands []SlashCommand, commandName string) *SlashCommand {
	for i := range commands {
		if commandMatchesNameOrAlias(commands[i], commandName) {
			return &commands[i]
		}
	}
	return nil
}
