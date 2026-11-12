package ops

import (
	"context"
	"errors"
	"testing"
)

func TestApplySelectionPoliciesDeduplicateAndRetainPartialRemainder(t *testing.T) {
	plan := DeliveryPlan{Apply: ApplyList{Teams: []ApplyTeam{{Team: "demo", Members: []ApplyMember{{Name: "demo-a", Short: "a"}, {Name: "demo-b", Short: "b"}}}}}, Members: map[string]PlannedMember{
		"demo-a": {Changed: true, How: ApplyStart}, "demo-b": {Changed: true, How: ApplyCannot, WhyNot: "working"},
	}}
	strict, err := SelectApply(plan, ApplySelectionInput{Policy: RequireWholeSelection, Names: []string{"demo-a", "demo-a", "demo-b", "demo-b"}})
	if err != nil || len(strict.Selected) != 1 || len(strict.Refusals) != 1 {
		t.Fatalf("strict: %+v, %v", strict, err)
	}
	partial, err := SelectApply(plan, ApplySelectionInput{Policy: ContinueAvailable})
	if err != nil || len(partial.Selected) != 1 || len(partial.Remaining) != 1 || len(partial.Refusals) != 0 {
		t.Fatalf("partial: %+v, %v", partial, err)
	}
}

func TestOperationLocksReserveEveryTeamAndCancelTheWholeOperation(t *testing.T) {
	locks := &OperationLocks{}
	ctx, release, err := locks.acquire(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	for _, name := range []string{"first", "second"} {
		if _, _, err := locks.acquire(context.Background(), []string{name}); err == nil {
			t.Errorf("%s was not reserved", name)
		}
	}
	locks.CancelTeam("second")
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatalf("the other team did not cancel the operation: %v", ctx.Err())
	}
	release()
	_, releaseAgain, err := locks.acquire(context.Background(), []string{"first", "second"})
	if err != nil {
		t.Fatal(err)
	}
	releaseAgain()
}
