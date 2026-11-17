package acl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestGrantConditionBind(t *testing.T) {
	midnight := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	afternoon := time.Date(2026, 11, 16, 15, 4, 5, 0, time.UTC)
	isDate := store.PropPredicate{Property: "until", Op: store.PropIsDate}
	ge := func(v string) store.PropPredicate {
		return store.PropPredicate{Property: "until", Op: store.PropGreaterEqual, Value: v}
	}
	le := func(v string) store.PropPredicate {
		return store.PropPredicate{Property: "until", Op: store.PropLessEqual, Value: v}
	}
	term := func(op GrantOp, operand GrantOperand) GrantTerm {
		return GrantTerm{Property: "until", Op: op, Operand: operand, Date: true}
	}
	tests := []struct {
		name string
		term GrantTerm
		at   time.Time
		want []store.PropPredicate
		ok   bool
	}{
		{"ge today", term(GrantGE, OperandToday), afternoon, []store.PropPredicate{isDate, ge("2026-11-16")}, true},
		{"gt today", term(GrantGT, OperandToday), afternoon, []store.PropPredicate{isDate, ge("2026-11-17")}, true},
		{"le today", term(GrantLE, OperandToday), afternoon, []store.PropPredicate{isDate, le("2026-11-16")}, true},
		{"lt today", term(GrantLT, OperandToday), afternoon, []store.PropPredicate{isDate, le("2026-11-15")}, true},
		{"eq today", term(GrantEq, OperandToday), afternoon,
			[]store.PropPredicate{isDate, ge("2026-11-16"), le("2026-11-16")}, true},
		{"ge now afternoon", term(GrantGE, OperandNow), afternoon, []store.PropPredicate{isDate, ge("2026-11-17")}, true},
		{"ge now midnight", term(GrantGE, OperandNow), midnight, []store.PropPredicate{isDate, ge("2026-11-16")}, true},
		{"lt now afternoon", term(GrantLT, OperandNow), afternoon, []store.PropPredicate{isDate, le("2026-11-16")}, true},
		{"lt now midnight", term(GrantLT, OperandNow), midnight, []store.PropPredicate{isDate, le("2026-11-15")}, true},
		{"le now afternoon", term(GrantLE, OperandNow), afternoon, []store.PropPredicate{isDate, le("2026-11-16")}, true},
		{"eq now afternoon never matches", term(GrantEq, OperandNow), afternoon, nil, false},
		{"zone is UTC", term(GrantGE, OperandToday),
			time.Date(2026, 11, 17, 0, 30, 0, 0, time.FixedZone("CET", 3600)),
			[]store.PropPredicate{isDate, ge("2026-11-16")}, true},
		{"literal ignores the read time", GrantTerm{
			Property: "until", Op: GrantGE, Operand: OperandLiteral, Literal: "2026-12-31", Date: true,
		}, time.Time{}, []store.PropPredicate{isDate, ge("2026-12-31")}, true},
		{"no read time denies", term(GrantGE, OperandToday), time.Time{}, nil, false},
		{"string equality", GrantTerm{Property: "kind", Op: GrantEq, Literal: "audit"}, afternoon,
			[]store.PropPredicate{{Property: "kind", Op: store.PropEqual, Value: "audit", Scalar: true}}, true},
		{"ordered string is refused", GrantTerm{Property: "kind", Op: GrantGE, Literal: "a"}, afternoon, nil, false},
		{"empty string literal is refused", GrantTerm{Property: "kind", Op: GrantEq}, afternoon, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := (&GrantCondition{Terms: []GrantTerm{tc.term}}).Bind(tc.at)
			require.Equal(t, tc.ok, ok)
			if ok {
				require.Equal(t, tc.want, got)
			}
		})
	}
}

func TestGrantConditionBind_OneGuardPerProperty(t *testing.T) {
	c := &GrantCondition{Terms: []GrantTerm{
		{Property: "from", Op: GrantLE, Operand: OperandToday, Date: true},
		{Property: "until", Op: GrantGE, Operand: OperandToday, Date: true},
		{Property: "until", Op: GrantLE, Operand: OperandLiteral, Literal: "2027-01-01", Date: true},
	}}
	got, ok := c.Bind(time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC))
	require.True(t, ok)
	require.Equal(t, []store.PropPredicate{
		{Property: "from", Op: store.PropIsDate},
		{Property: "from", Op: store.PropLessEqual, Value: "2026-11-16"},
		{Property: "until", Op: store.PropIsDate},
		{Property: "until", Op: store.PropGreaterEqual, Value: "2026-11-16"},
		{Property: "until", Op: store.PropLessEqual, Value: "2027-01-01"},
	}, got)
}

func TestCheckGrantConditions(t *testing.T) {
	cond := &GrantCondition{Terms: []GrantTerm{{Property: "until", Op: GrantGE, Operand: OperandToday, Date: true}}}
	tests := []struct {
		name    string
		def     RoleRelationDef
		wantErr string
	}{
		{"unconditional", RoleRelationDef{Confers: "r"}, ""},
		{"compiled and gated", RoleRelationDef{Confers: "r", RequiresPermission: "p", When: "x", Condition: cond}, ""},
		{"not compiled", RoleRelationDef{Confers: "r", RequiresPermission: "p", When: "x"}, "was not compiled"},
		{"ungated", RoleRelationDef{Confers: "r", When: "x", Condition: cond}, "requires_permission"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkGrantConditions(&Policy{RoleRelations: map[string]RoleRelationDef{"grants": tc.def}})
			if tc.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}

func TestBindGrantConds_UncompiledWhenFailsClosed(t *testing.T) {
	p := &Policy{RoleRelations: map[string]RoleRelationDef{
		"grants": {Confers: "auditor", When: "edge.valid_until >= today()"},
		"owns":   {Confers: "auditor"},
	}}
	conds := bindGrantConds(p, time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC))
	if _, ok := conds.edge("grants"); ok {
		t.Error("a when: without a compiled condition must count no edge")
	}
	if preds, ok := conds.edge("owns"); !ok || preds != nil {
		t.Errorf("an unconditional relation counts every edge: preds=%v ok=%v", preds, ok)
	}
}
