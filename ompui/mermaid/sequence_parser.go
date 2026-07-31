// Ported from mermaid-ascii 1.4.0 (github.com/AlexanderGrooff/mermaid-ascii),
// commit 823db562a4439e342541643bbd5cb7d75c930e8e, MIT License
// Copyright (c) 2023 Alexander Grooff.
// Bounded pure graph/flowchart + sequenceDiagram port for ompui; no cmd/web,
// cobra, gin, logrus, or gookit/color dependencies.

package mermaid

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	SequenceDiagramKeyword = "sequenceDiagram"
	SolidArrowSyntax       = "->>"
	DottedArrowSyntax      = "-->>"
)

var (
	// participantRegex matches participant declarations: participant [ID] [as Label]
	participantRegex = regexp.MustCompile(`^\s*participant\s+(?:"([^"]+)"|(\S+))(?:\s+as\s+(.+))?$`)

	// messageRegex matches messages: [From]->>[To]: [Label]
	messageRegex = regexp.MustCompile(`^\s*(?:"([^"]+)"|([^\s\->]+))\s*(-->>|->>)\s*(?:"([^"]+)"|([^\s\->]+))\s*:\s*(.*)$`)

	// autonumberRegex matches the autonumber directive
	autonumberRegex = regexp.MustCompile(`^\s*autonumber\s*$`)

	// fragmentStartRegex matches the opening line of a control-flow fragment,
	// e.g. "loop every minute" or "opt is premium". Group 1 is the keyword,
	// group 2 is the (optional) label describing the condition.
	fragmentStartRegex = regexp.MustCompile(`^\s*(loop|opt|alt|else|par|and)\b\s*(.*)$`)

	// fragmentEndRegex matches the "end" line that closes a fragment.
	fragmentEndRegex = regexp.MustCompile(`^\s*end\s*$`)

	// noteRegex matches note over/left/right of participant(s).
	// Forms: note left of A: text | note right of A: text | note over A: text | note over A,B: text
	noteRegex = regexp.MustCompile(`(?i)^\s*note\s+(left of|right of|over)\s+(.+)$`)
)

// SequenceDiagram represents a parsed sequence diagram.
type SequenceDiagram struct {
	Participants []*Participant
	// Messages is the flat list of every message arrow, in source order and
	// independent of any fragment nesting. Kept for callers that only care
	// about the messages themselves (and for backward compatibility).
	Messages []*Message
	// Events is the ordered body of the diagram used for rendering: each entry
	// is either a message or a fragment boundary. Walking Events reproduces the
	// original source order, including where loop/opt blocks open and close.
	Events     []Event
	Autonumber bool
}

// FragmentType identifies a control-flow fragment (a "framed" block of
// messages) such as a loop or an optional section.
type FragmentType int

const (
	FragmentLoop FragmentType = iota // loop ... end
	FragmentOpt                      // opt ... end
	FragmentAlt                      // alt ... else ... end
	FragmentElse                     // else branch inside alt (rendered as nested label tab)
	FragmentPar                      // par ... and ... end
	FragmentAnd                      // and branch inside par
)

func (f FragmentType) String() string {
	switch f {
	case FragmentLoop:
		return "loop"
	case FragmentOpt:
		return "opt"
	case FragmentAlt:
		return "alt"
	case FragmentElse:
		return "else"
	case FragmentPar:
		return "par"
	case FragmentAnd:
		return "and"
	default:
		return fmt.Sprintf("FragmentType(%d)", int(f))
	}
}

// Fragment describes the opening of a control-flow block: its kind and the
// optional condition text shown in the frame's label tab.
type Fragment struct {
	Type  FragmentType
	Label string
}

// EventKind tags each Event in the diagram body.
type EventKind int

const (
	EventMessage       EventKind = iota // a message arrow
	EventFragmentStart                  // the opening line of a loop/opt/alt/par block
	EventFragmentEnd                    // the matching "end" line
	EventNote                           // a note over/beside participants
)

// Event is one item in the diagram body. Exactly one payload field is set:
// Message when Kind is EventMessage, Fragment when Kind is EventFragmentStart.
// An EventFragmentEnd carries no payload; it just marks where a block closes.
type Event struct {
	Kind     EventKind
	Message  *Message
	Fragment *Fragment
	Note     *Note
}

// NotePlacement is where a note sits relative to participants.
type NotePlacement int

const (
	NoteOver NotePlacement = iota
	NoteLeftOf
	NoteRightOf
)

// Note is a sequence-diagram annotation.
type Note struct {
	Placement NotePlacement
	From      *Participant // primary / left participant
	To        *Participant // optional right participant for "over A,B"
	Text      string
}

type Participant struct {
	ID    string
	Label string
	Index int
}

type Message struct {
	From      *Participant
	To        *Participant
	Label     string
	ArrowType ArrowType
	Number    int // Message number when autonumber is enabled (0 means no number)
}

type ArrowType int

const (
	SolidArrow ArrowType = iota
	DottedArrow
)

func (a ArrowType) String() string {
	switch a {
	case SolidArrow:
		return "solid"
	case DottedArrow:
		return "dotted"
	default:
		return fmt.Sprintf("ArrowType(%d)", a)
	}
}

func IsSequenceDiagram(input string) bool {
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "%%") {
			continue
		}
		return strings.HasPrefix(trimmed, SequenceDiagramKeyword)
	}
	return false
}

func ParseSequence(input string) (*SequenceDiagram, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("empty input")
	}

	rawLines := SplitLines(input)
	lines := RemoveComments(rawLines)
	if len(lines) == 0 {
		return nil, fmt.Errorf("no content found")
	}

	if !strings.HasPrefix(strings.TrimSpace(lines[0]), SequenceDiagramKeyword) {
		return nil, fmt.Errorf("expected %q keyword", SequenceDiagramKeyword)
	}
	lines = lines[1:]

	sd := &SequenceDiagram{
		Participants: []*Participant{},
		Messages:     []*Message{},
		Autonumber:   false,
	}
	participantMap := make(map[string]*Participant)
	// openFragments counts how many loop/opt blocks are currently open so we can
	// reject an "end" with no matching opener and, at the very end, an opener
	// with no matching "end".
	openFragments := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Check for autonumber directive
		if autonumberRegex.MatchString(trimmed) {
			sd.Autonumber = true
			continue
		}

		if matched, err := sd.parseParticipant(trimmed, participantMap); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+2, err)
		} else if matched {
			continue
		}

		// Messages are checked before fragment keywords so a participant named
		// "loop"/"opt"/"end" (e.g. "loop->>B: hi") is still read as a message —
		// only bare openers like "loop retry" fall through to the checks below.
		if matched, err := sd.parseMessage(trimmed, participantMap); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+2, err)
		} else if matched {
			continue
		}

		// A fragment opener starts a framed block (loop/opt/alt/par) or a
		// mid-fragment branch divider (else/and).
		if match := fragmentStartRegex.FindStringSubmatch(trimmed); match != nil {
			kw := strings.ToLower(match[1])
			label := strings.TrimSpace(match[2])
			switch kw {
			case "else", "and":
				// Treat branch dividers as closing the previous section and
				// opening a sibling fragment so the renderer can draw a labelled
				// frame for each branch. Require an open parent fragment.
				if openFragments == 0 {
					return nil, fmt.Errorf("line %d: %q without matching alt/par", i+2, trimmed)
				}
				sd.Events = append(sd.Events, Event{Kind: EventFragmentEnd})
				fType := FragmentElse
				if kw == "and" {
					fType = FragmentAnd
				}
				sd.Events = append(sd.Events, Event{
					Kind:     EventFragmentStart,
					Fragment: &Fragment{Type: fType, Label: label},
				})
				// openFragments unchanged: end+start
				continue
			default:
				fType := FragmentLoop
				switch kw {
				case "opt":
					fType = FragmentOpt
				case "alt":
					fType = FragmentAlt
				case "par":
					fType = FragmentPar
				}
				sd.Events = append(sd.Events, Event{
					Kind:     EventFragmentStart,
					Fragment: &Fragment{Type: fType, Label: label},
				})
				openFragments++
				continue
			}
		}

		// Notes: "note left of A: text" / "note over A,B: text"
		if match := noteRegex.FindStringSubmatch(trimmed); match != nil {
			note, err := sd.parseNote(match[1], match[2], participantMap)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+2, err)
			}
			sd.Events = append(sd.Events, Event{Kind: EventNote, Note: note})
			continue
		}

		// "end" closes the most recently opened fragment.
		if fragmentEndRegex.MatchString(trimmed) {
			if openFragments == 0 {
				return nil, fmt.Errorf("line %d: %q without matching fragment", i+2, trimmed)
			}
			sd.Events = append(sd.Events, Event{Kind: EventFragmentEnd})
			openFragments--
			continue
		}

		return nil, fmt.Errorf("line %d: invalid syntax: %q", i+2, trimmed)
	}

	if openFragments > 0 {
		return nil, fmt.Errorf("unclosed loop/opt fragment: missing %d \"end\"", openFragments)
	}

	if len(sd.Participants) == 0 {
		return nil, fmt.Errorf("no participants found")
	}

	return sd, nil
}

func (sd *SequenceDiagram) parseParticipant(line string, participants map[string]*Participant) (bool, error) {
	match := participantRegex.FindStringSubmatch(line)
	if match == nil {
		return false, nil
	}

	id := match[2]
	if match[1] != "" {
		id = match[1]
	}
	label := match[3]
	if label == "" {
		label = id
	}
	label = strings.Trim(label, `"`)

	if _, exists := participants[id]; exists {
		return true, fmt.Errorf("duplicate participant %q", id)
	}

	p := &Participant{
		ID:    id,
		Label: label,
		Index: len(sd.Participants),
	}
	sd.Participants = append(sd.Participants, p)
	participants[id] = p
	return true, nil
}

func (sd *SequenceDiagram) parseMessage(line string, participants map[string]*Participant) (bool, error) {
	match := messageRegex.FindStringSubmatch(line)
	if match == nil {
		return false, nil
	}

	fromID := match[2]
	if match[1] != "" {
		fromID = match[1]
	}

	arrow := match[3]

	toID := match[5]
	if match[4] != "" {
		toID = match[4]
	}

	label := strings.TrimSpace(match[6])

	from := sd.getParticipant(fromID, participants)
	to := sd.getParticipant(toID, participants)

	aType := DottedArrow
	if arrow == SolidArrowSyntax {
		aType = SolidArrow
	}

	msgNumber := 0
	if sd.Autonumber {
		msgNumber = len(sd.Messages) + 1
	}

	msg := &Message{
		From:      from,
		To:        to,
		Label:     label,
		ArrowType: aType,
		Number:    msgNumber,
	}
	sd.Messages = append(sd.Messages, msg)
	sd.Events = append(sd.Events, Event{Kind: EventMessage, Message: msg})
	return true, nil
}

func (sd *SequenceDiagram) getParticipant(id string, participants map[string]*Participant) *Participant {
	if p, exists := participants[id]; exists {
		return p
	}

	p := &Participant{
		ID:    id,
		Label: id,
		Index: len(sd.Participants),
	}
	sd.Participants = append(sd.Participants, p)
	participants[id] = p
	return p
}

func (sd *SequenceDiagram) parseNote(place, rest string, participants map[string]*Participant) (*Note, error) {
	place = strings.ToLower(strings.TrimSpace(place))
	// rest: "A: text" or "A,B: text"
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("note requires text after ':'")
	}
	who := strings.TrimSpace(parts[0])
	text := strings.TrimSpace(parts[1])
	if text == "" {
		return nil, fmt.Errorf("empty note text")
	}
	var placement NotePlacement
	switch place {
	case "left of":
		placement = NoteLeftOf
	case "right of":
		placement = NoteRightOf
	default:
		placement = NoteOver
	}
	// participants may be "A" or "A,B"
	ids := strings.Split(who, ",")
	for i := range ids {
		ids[i] = strings.TrimSpace(strings.Trim(ids[i], `"`))
	}
	if ids[0] == "" {
		return nil, fmt.Errorf("note missing participant")
	}
	from := sd.getParticipant(ids[0], participants)
	var to *Participant
	if len(ids) > 1 && ids[1] != "" {
		to = sd.getParticipant(ids[1], participants)
	}
	return &Note{Placement: placement, From: from, To: to, Text: text}, nil
}
