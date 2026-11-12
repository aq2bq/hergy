package name

// Naming chooses how prompts and checks spell names. A running team carries a
// TeamRef (a checked name or a legacy observation); a definition preview carries its existing directory name.
// Only the constructors can choose a branch.
type Naming struct {
	team       TeamRef
	definition DefinitionName
}

// RunningNames names members of a running team.
func RunningNames(team TeamRef) Naming { return Naming{team: team} }

// DefinitionNames names the prospective members of a definition.
func DefinitionNames(definition DefinitionName) Naming { return Naming{definition: definition} }

func (n Naming) String() string {
	if n.team.String() != "" {
		return n.team.String()
	}
	return n.definition.s
}

// Member is the spelling printed for a member, including diagnostics for a
// composed name that cannot be started. Actual targets still require MemberTarget.
func (n Naming) Member(member MemberName) string {
	return targetSpelling(n.String(), member.s)
}

// Worker is a preview spelling. The short part may be a UI placeholder and is
// never made into a herdr target.
func (n Naming) Worker(short string) string { return targetSpelling(n.String(), short) }

// MemberTarget checks the prospective or running member using the same rule.
func (n Naming) MemberTarget(member MemberName) (HerdrName, error) {
	t, model := n.team.Model()
	if !model {
		var err error
		if n.team.String() != "" {
			return HerdrName{}, &Problem{Kind: Herdr, Value: n.Member(member), Max: 32}
		}
		t, err = n.definition.Team()
		if err != nil {
			return HerdrName{}, &Problem{Kind: Herdr, Value: n.Member(member), Max: 32}
		}
	}
	return t.Member(member)
}

// Observation retains a checked name when passing it to read-only operations.
func (n TeamName) Observation() Seen { return Seen(n) }

// Observation retains the constructed target for read-only operations.
func (n HerdrName) Observation() Seen { return Seen(n) }

// Observation retains the checked short member spelling.
func (n MemberName) Observation() Seen { return Seen(n) }

// Observation retains a checked short target spelling.
func (n ShortName) Observation() Seen { return Seen(n) }

// WorkerTarget checks a real worker, as distinct from a preview placeholder.
func (n Naming) WorkerTarget(worker WorkerName) (HerdrName, error) {
	t, model := n.team.Model()
	if !model {
		var err error
		if n.team.String() != "" {
			return HerdrName{}, &Problem{Kind: Herdr, Value: n.Worker(worker.s), Max: 32}
		}
		t, err = n.definition.Team()
		if err != nil {
			return HerdrName{}, &Problem{Kind: Herdr, Value: n.Worker(worker.s), Max: 32}
		}
	}
	return t.Worker(worker)
}

// Ref is the model branch of a running team identity.
func (n TeamName) Ref() TeamRef { return TeamRef{model: n} }

// WorkerTab is a display-only prospective tab label.
func (n Naming) WorkerTab(short string) string { return workerLabel(n.String(), short) }

// Short reads a displayed or recorded name in this naming context. Unmatched
// text is retained. The result is display text, never a target.
func (n Naming) Short(raw string) string {
	return ShortIn(Seen{s: raw}, Seen{s: n.String()}).String()
}

// Contains reports whether a writer's spelling belongs to this context.
func (n Naming) Contains(raw string) bool {
	return BelongsTo(Seen{s: raw}, Seen{s: n.String()})
}

// NamingSet is the naming contexts a recorded writer may belong to.
type NamingSet []Naming

// Resolve reads the longest matching context. Its short result is display text,
// so a ledger writer cannot become a herdr target through this operation.
func (all NamingSet) Resolve(raw string) (Naming, string, bool) {
	teams := make([]Seen, len(all))
	for i, n := range all {
		teams[i] = Seen{s: n.String()}
	}
	owner, short, ok := Resolve(Seen{s: raw}, teams)
	if !ok {
		return Naming{}, "", false
	}
	for _, n := range all {
		if n.String() == owner.String() {
			return n, short.String(), true
		}
	}
	return Naming{}, "", false
}

// DisplayMember retains an already prefixed effect/record line. Unlike Member,
// its input is display text and its result cannot be used as a target.
func (n Naming) DisplayMember(raw string) string {
	if n.Contains(raw) {
		return raw
	}
	return targetSpelling(n.String(), raw)
}
