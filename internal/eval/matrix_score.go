// The scorecard: what a cell claims, what each claim is worth, and how the claims add up.
//
// A cell is not one question. Bringing a sandbox up, reaching the guest, rendering the page,
// leaving the change behind and showing that the integration level itself carried the work are
// separate claims, and a single pass/fail reports the whole scenario by its weakest part while
// saying nothing about which part that was.
package eval

// A cell is not one question. Bringing a sandbox up, reaching the guest, rendering the page,
// leaving the change behind and showing that the integration level itself carried the work are
// separate claims, and a single pass/fail reports the whole scenario by its weakest part while
// telling you nothing about which part that was. Each claim is scored on its own, so a cell that
// does nine tenths of the job reads as nine tenths rather than as a failure indistinguishable from
// a sandbox that never started.
type check struct {
	Name   string
	Weight int // a claim about containment counts for more than one about tidiness
	Passed bool
	Detail string // why not, when not
}

// checks is one cell's scorecard.
type checks []check

func (cs checks) score() (got, total int) {
	for _, c := range cs {
		total += c.Weight
		if c.Passed {
			got += c.Weight
		}
	}
	return got, total
}

func (cs checks) percent() float64 {
	got, total := cs.score()
	if total == 0 {
		return 0
	}
	return 100 * float64(got) / float64(total)
}

func (cs checks) failed() []string {
	var out []string
	for _, c := range cs {
		if !c.Passed {
			out = append(out, c.Name+" ("+c.Detail+")")
		}
	}
	return out
}
