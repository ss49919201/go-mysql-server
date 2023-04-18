// Copyright 2020-2021 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package analyzer

import (
	"fmt"
	"github.com/dolthub/go-mysql-server/sql/expression/function"
	"github.com/dolthub/vitess/go/mysql"
	"reflect"
	"strings"

	"github.com/dolthub/go-mysql-server/sql"
	"github.com/dolthub/go-mysql-server/sql/analyzer/analyzererrors"
	"github.com/dolthub/go-mysql-server/sql/expression"
	"github.com/dolthub/go-mysql-server/sql/expression/function/aggregation"
	"github.com/dolthub/go-mysql-server/sql/plan"
	"github.com/dolthub/go-mysql-server/sql/transform"
	"github.com/dolthub/go-mysql-server/sql/types"
)

var ErrInvalidCheck = fmt.Errorf("error validating check")

type ValidatorFunc = func(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector)

type semError struct{ error }

var validationRulesBefore = []func(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector){
	validateLimitAndOffset,
	validateDeleteFrom,
	validatePrivileges,
	validateCreateTable,
	validateExprSem,
	validateDropConstraint,
	validateReadOnlyDatabase,
	validateReadOnlyTransaction,
	//validateColumnDefaults,
	validateDatabaseSet,
}

// validateAll runs all validation rules
func validateBefore(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) (ret sql.Node, same transform.TreeIdentity, err error) {
	ret = n
	same = transform.SameTree
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case semError:
				err = e.error
			default:
				panic(e)
			}
		}
	}()
	for _, v := range validationRulesBefore {
		v(ctx, a, n, scope, sel)
	}
	return
}

var validationRulesAfter = []func(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector){
	validateIsResolved,
	validateOrderBy,
	validateGroupBy,
	validateSchemaSource,
	validateIndexCreation,
	validateIntervalUsage,
	validateOperands,
	validateSubqueryColumns,
	validateAggregations,
}

// validateAll runs all validation rules
func validateAfter(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) (ret sql.Node, same transform.TreeIdentity, err error) {
	ret = n
	same = transform.SameTree
	defer func() {
		if r := recover(); r != nil {
			switch e := r.(type) {
			case semError:
				err = e.error
			default:
				panic(e)
			}
		}
	}()
	for _, v := range validationRulesAfter {
		v(ctx, a, n, scope, sel)
	}
	return
}

// validateDatabaseSet returns an error if any database node that requires a database doesn't have one
func validateDatabaseSet(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	transform.Inspect(n, func(n sql.Node) bool {
		switch n.(type) {
		// TODO: there are probably other kinds of nodes that need this too
		case *plan.ShowTables, *plan.ShowTriggers, *plan.CreateTable:
			n := n.(sql.Databaser)
			if _, ok := n.Database().(sql.UnresolvedDatabase); ok {
				panic(semError{sql.ErrNoDatabaseSelected.New()})
			}
		}
		return true
	})
	return
}

// validateDropConstraint returns an error if the constraint named to be dropped doesn't exist
func validateDropConstraint(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	switch n := n.(type) {
	case *plan.DropCheck:
		rt, ok := n.Child.(*plan.ResolvedTable)
		if !ok {
			panic(semError{ErrInAnalysis.New("Expected a ResolvedTable for ALTER TABLE DROP CONSTRAINT statement")})
		}

		ct, ok := rt.Table.(sql.CheckTable)
		if ok {
			checks, err := ct.GetChecks(ctx)
			if err != nil {
				panic(semError{err})
			}

			for _, check := range checks {
				if strings.ToLower(check.Name) == strings.ToLower(n.Name) {
					return
				}
			}

			panic(semError{sql.ErrUnknownConstraint.New(n.Name)})
		}

		panic(semError{plan.ErrNoCheckConstraintSupport.New(rt.Table.Name())})
	default:
		return
	}
}

// validateCreateTable validates various constraints about CREATE TABLE statements. Some validation is currently done
// at execution time, and should be moved here over time.
func validateCreateTable(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	ct, ok := n.(*plan.CreateTable)
	if !ok {
		return
	}

	validateIndexes(ctx, ct.TableSpec())

	// passed validateIndexes, so they all must be valid indexes
	// extract map of columns that have indexes defined over them
	keyedColumns := make(map[string]bool)
	for _, index := range ct.TableSpec().IdxDefs {
		for _, col := range index.Columns {
			keyedColumns[col.Name] = true
		}
	}

	if err := validateAutoIncrement(ct.CreateSchema.Schema, keyedColumns); err != nil {
		panic(semError{err})
	}
}

// validateLimitAndOffset ensures that only integer literals are used for limit and offset values
func validateLimitAndOffset(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	var err error
	var i, i64 interface{}
	transform.Inspect(n, func(n sql.Node) bool {
		switch n := n.(type) {
		case *plan.Limit:
			switch e := n.Limit.(type) {
			case *expression.Literal:
				if !types.IsInteger(e.Type()) {
					panic(semError{sql.ErrInvalidType.New(e.Type().String())})
				}
				i, err = e.Eval(ctx, nil)
				if err != nil {
					panic(semError{err})
				}

				i64, _, err = types.Int64.Convert(i)
				if err != nil {
					return false
				}
				if i64.(int64) < 0 {
					panic(semError{sql.ErrInvalidSyntax.New("negative limit")})
				}
			case *expression.BindVar:
			default:
				panic(semError{sql.ErrInvalidType.New(e.Type().String())})
			}
		case *plan.Offset:
			switch e := n.Offset.(type) {
			case *expression.Literal:
				if !types.IsInteger(e.Type()) {
					panic(semError{sql.ErrInvalidType.New(e.Type().String())})
				}
				i, err = e.Eval(ctx, nil)
				if err != nil {
					panic(semError{err})
				}
				i64, _, err = types.Int64.Convert(i)
				if err != nil {
					panic(semError{err})
				}
				if i64.(int64) < 0 {
					panic(semError{sql.ErrInvalidSyntax.New("negative offset")})
				}
			case *expression.BindVar:
			default:
				panic(semError{sql.ErrInvalidType.New(e.Type().String())})
			}
		default:
		}
		return true
	})
}

func validateIsResolved(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	span, ctx := ctx.Span("validate_is_resolved")
	defer span.End()

	if !n.Resolved() {
		unresolvedError(n)
	}
}

// unresolvedError returns an appropriate error message for the unresolved node given
func unresolvedError(n sql.Node) {
	var err error
	var walkFn func(sql.Expression) bool
	walkFn = func(e sql.Expression) bool {
		switch e := e.(type) {
		case *plan.Subquery:
			transform.InspectExpressions(e.Query, walkFn)
			if err != nil {
				return false
			}
		case *deferredColumn:
			if e.Table() != "" {
				panic(semError{sql.ErrTableColumnNotFound.New(e.Table(), e.Name())})
			} else {
				panic(semError{sql.ErrColumnNotFound.New(e.Name())})
			}
			return false
		}
		return true
	}
	transform.InspectExpressions(n, walkFn)
	panic(semError{analyzererrors.ErrValidationResolved.New(n)})
}

func validateOrderBy(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	span, ctx := ctx.Span("validate_order_by")
	defer span.End()

	switch n := n.(type) {
	case *plan.Sort:
		for _, field := range n.SortFields {
			switch field.Column.(type) {
			case sql.Aggregation:
				panic(semError{analyzererrors.ErrValidationOrderBy.New()})
			}
		}
	}
}

// validateDeleteFrom checks for invalid settings, such as deleting from multiple databases, specifying a delete target
// table multiple times, or using a DELETE FROM JOIN without specifying any explicit delete target tables, and returns
// an error if any validation issues were detected.
func validateDeleteFrom(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	span, ctx := ctx.Span("validate_order_by")
	defer span.End()
	transform.InspectUp(n, func(n sql.Node) bool {
		df, ok := n.(*plan.DeleteFrom)
		if !ok {
			return false
		}

		// Check that delete from join only targets tables that exist in the join
		if df.HasExplicitTargets() {
			sourceTables := make(map[string]struct{})
			transform.Inspect(df.Child, func(node sql.Node) bool {
				if t, ok := node.(sql.Table); ok {
					sourceTables[t.Name()] = struct{}{}
				}
				return true
			})

			for _, target := range df.GetDeleteTargets() {
				deletable, err := plan.GetDeletable(target)
				if err != nil {
					panic(semError{err})
				}
				tableName := deletable.Name()
				if _, ok := sourceTables[tableName]; !ok {
					panic(semError{fmt.Errorf("table %q not found in DELETE FROM sources", tableName)})
				}
			}
		}

		// Duplicate explicit target tables or from explicit target tables from multiple databases
		databases := make(map[string]struct{})
		tables := make(map[string]struct{})
		if df.HasExplicitTargets() {
			for _, target := range df.GetDeleteTargets() {
				// Check for multiple databases
				databases[plan.GetDatabaseName(target)] = struct{}{}
				if len(databases) > 1 {
					panic(semError{fmt.Errorf("multiple databases specified as delete from targets")})
					return true
				}

				// Check for duplicate targets
				nameable, ok := target.(sql.Nameable)
				if !ok {
					panic(semError{fmt.Errorf("target node does not implement sql.Nameable: %T", target)})
					return true
				}

				if _, ok := tables[nameable.Name()]; ok {
					panic(semError{fmt.Errorf("duplicate tables specified as delete from targets")})
					return true
				}
				tables[nameable.Name()] = struct{}{}
			}
		}

		transform.Inspect(df.Child, func(node sql.Node) bool {
			if _, ok := node.(*plan.JoinNode); ok && !df.HasExplicitTargets() {
				panic(semError{fmt.Errorf("delete from statement with join requires specifying explicit delete target tables")})
			}
			return true
		})
		return false
	})
}

// checkSqlMode checks if the option is set for the Session in ctx
func checkSqlMode(ctx *sql.Context, option string) bool {
	// session variable overrides global
	sysVal, err := ctx.Session.GetSessionVariable(ctx, "sql_mode")
	if err != nil {
		panic(semError{err})
	}
	val, ok := sysVal.(string)
	if !ok {
		panic(semError{sql.ErrSystemVariableCodeFail.New("sql_mode", val)})
	}
	return strings.Contains(val, option)
}

func validateGroupBy(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	span, ctx := ctx.Span("validate_group_by")
	defer span.End()

	// only enforce strict group by when this variable is set
	if isStrict := checkSqlMode(ctx, "ONLY_FULL_GROUP_BY"); !isStrict {
		return
	}

	var parent sql.Node
	transform.Inspect(n, func(n sql.Node) bool {
		defer func() {
			parent = n
		}()

		gb, ok := n.(*plan.GroupBy)
		if !ok {
			return true
		}

		switch parent.(type) {
		case *plan.Having, *plan.Project, *plan.Sort:
			// TODO: these shouldn't be skipped; you can group by primary key without problem b/c only one value
			// https://dev.mysql.com/doc/refman/8.0/en/group-by-handling.html#:~:text=The%20query%20is%20valid%20if%20name%20is%20a%20primary%20key
			return true
		}

		// Allow the parser use the GroupBy node to eval the aggregation functions
		// for sql statements that don't make use of the GROUP BY expression.
		if len(gb.GroupByExprs) == 0 {
			return true
		}

		var groupBys []string
		for _, expr := range gb.GroupByExprs {
			groupBys = append(groupBys, expr.String())
		}

		for _, expr := range gb.SelectedExprs {
			if _, ok := expr.(sql.Aggregation); !ok {
				if !expressionReferencesOnlyGroupBys(groupBys, expr) {
					panic(semError{analyzererrors.ErrValidationGroupBy.New(expr.String())})
					return false
				}
			}
		}
		return true
	})
}

func stringContains(strs []string, target string) bool {
	for _, s := range strs {
		if s == target {
			return true
		}
	}
	return false
}

func tableColsContains(strs []tableCol, target tableCol) bool {
	for _, s := range strs {
		if s == target {
			return true
		}
	}
	return false
}

func expressionReferencesOnlyGroupBys(groupBys []string, expr sql.Expression) bool {
	valid := true
	sql.Inspect(expr, func(expr sql.Expression) bool {
		switch expr := expr.(type) {
		case nil, sql.Aggregation, *expression.Literal:
			return false
		case *expression.Alias, sql.FunctionExpression:
			if stringContains(groupBys, expr.String()) {
				return false
			}
			return true
		// cc: https://dev.mysql.com/doc/refman/8.0/en/group-by-handling.html
		// Each part of the SelectExpr must refer to the aggregated columns in some way
		// TODO: this isn't complete, it's overly restrictive. Dependant columns are fine to reference.
		default:
			if stringContains(groupBys, expr.String()) {
				return true
			}

			if len(expr.Children()) == 0 {
				valid = false
				return false
			}

			return true
		}
	})

	return valid
}

func validateSchemaSource(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	validateSchema2 := func(t *plan.ResolvedTable) {
		for _, col := range t.Schema() {
			if col.Source == "" {
				panic(semError{analyzererrors.ErrValidationSchemaSource.New()})
			}
		}
	}
	switch n := n.(type) {
	case *plan.TableAlias:
		// table aliases should not be validated
		if child, ok := n.Child.(*plan.ResolvedTable); ok {
			validateSchema2(child)
		}
	case *plan.ResolvedTable:
		validateSchema2(n)
	}
}

func validateIndexCreation(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	span, ctx := ctx.Span("validate_index_creation")
	defer span.End()

	ci, ok := n.(*plan.CreateIndex)
	if !ok {
		return
	}

	schema := ci.Table.Schema()
	table := schema[0].Source

	var unknownColumns []string
	for _, expr := range ci.Exprs {
		sql.Inspect(expr, func(e sql.Expression) bool {
			gf, ok := e.(*expression.GetField)
			if ok {
				if gf.Table() != table || !schema.Contains(gf.Name(), gf.Table()) {
					unknownColumns = append(unknownColumns, gf.Name())
				}
			}
			return true
		})
	}

	if len(unknownColumns) > 0 {
		panic(semError{analyzererrors.ErrUnknownIndexColumns.New(table, strings.Join(unknownColumns, ", "))})
	}
}

func validateSchema2(t *plan.ResolvedTable) {
	for _, col := range t.Schema() {
		if col.Source == "" {
			panic(semError{analyzererrors.ErrValidationSchemaSource.New()})
		}
	}
}

func validateUnionSchemasMatch(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) (sql.Node, transform.TreeIdentity, error) {
	span, ctx := ctx.Span("validate_union_schemas_match")
	defer span.End()

	var firstmismatch []string
	transform.Inspect(n, func(n sql.Node) bool {
		if u, ok := n.(*plan.Union); ok {
			ls := u.Left().Schema()
			rs := u.Right().Schema()
			if len(ls) != len(rs) {
				firstmismatch = []string{
					fmt.Sprintf("%d columns", len(ls)),
					fmt.Sprintf("%d columns", len(rs)),
				}
				return false
			}
			for i := range ls {
				if !reflect.DeepEqual(ls[i].Type, rs[i].Type) {
					firstmismatch = []string{
						ls[i].Type.String(),
						rs[i].Type.String(),
					}
					return false
				}
			}
		}
		return true
	})
	if firstmismatch != nil {
		return nil, transform.SameTree, analyzererrors.ErrUnionSchemasMatch.New(firstmismatch[0], firstmismatch[1])
	}
	return n, transform.SameTree, nil
}

func validateIntervalUsage(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	var invalid bool
	transform.InspectExpressionsWithNode(n, func(node sql.Node, e sql.Expression) bool {
		// If it's already invalid just skip everything else.
		if invalid {
			return false
		}

		// Interval can be used without DATE_ADD/DATE_SUB functions in CREATE/ALTER EVENTS statements.
		switch node.(type) {
		case *plan.CreateEvent:
			return false
		}

		switch e := e.(type) {
		case *function.DateAdd, *function.DateSub:
			return false
		case *expression.Arithmetic:
			if e.Op == "+" || e.Op == "-" {
				return false
			}
		case *expression.Interval:
			invalid = true
		}
		return true

	})
	if invalid {
		panic(semError{analyzererrors.ErrIntervalInvalidUse.New()})
	}

	return
}

func validateStarExpressions(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) (sql.Node, transform.TreeIdentity, error) {
	// Validate that all occurences of the '*' placeholder expression are in a context that makes sense.
	//
	// That is, all uses of '*' should be either:
	// - The top level of an expression.
	// - The input to a COUNT or JSONARRAY function.
	//
	// We do not use plan.InspectExpressions here because we're treating
	// the top-level expressions of sql.Node differently from subexpressions.
	var err error
	transform.Inspect(n, func(n sql.Node) bool {
		if er, ok := n.(sql.Expressioner); ok {
			for _, e := range er.Expressions() {
				// An expression consisting of just a * is allowed.
				if _, s := e.(*expression.Star); s {
					return false
				}
				// Otherwise, * can only be used inside acceptable aggregation functions.
				// Detect any uses of * outside such functions.
				sql.Inspect(e, func(e sql.Expression) bool {
					if err != nil {
						return false
					}
					switch e.(type) {
					case *expression.Star:
						err = analyzererrors.ErrStarUnsupported.New()
						return false
					case *aggregation.Count, *aggregation.CountDistinct, *aggregation.JsonArray:
						if _, s := e.Children()[0].(*expression.Star); s {
							return false
						}
					}
					return true
				})
			}
		}
		return err == nil
	})
	if err != nil {
		return nil, transform.SameTree, err
	}
	return n, transform.SameTree, nil
}

func validateOperands(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	// Validate that the number of columns in an operand or a top level
	// expression are as expected. The current rules are:
	// * Every top level expression of a node must have 1 column.
	// * The following expression nodes are allowed to have `n` columns as
	// long as `n` matches:
	//   * *plan.InSubquery, *expression.{Equals,NullSafeEquals,GreaterThan,LessThan,GreaterThanOrEqual,LessThanOrEqual}
	// * *expression.InTuple must have a tuple on the right side, the # of
	// columns for each element of the tuple must match the number of
	// columns of the expression on the left.
	// * Every other expression with operands must have NumColumns == 1.

	// We do not use plan.InspectExpressions here because we're treating
	// top-level expressions of sql.Node differently from subexpressions.
	transform.Inspect(n, func(n sql.Node) bool {
		if n == nil {
			return false
		}

		if plan.IsDDLNode(n) {
			return false
		}

		if er, ok := n.(sql.Expressioner); ok {
			for _, e := range er.Expressions() {
				nc := types.NumColumns(e.Type())
				if nc != 1 {
					if _, ok := er.(*plan.HashLookup); ok {
						// hash lookup expressions are tuples with >= 1 columns
						return true
					}
					panic(semError{sql.ErrInvalidOperandColumns.New(1, nc)})
				}
				sql.Inspect(e, func(e sql.Expression) bool {
					if e == nil {
						return true
					}
					switch e.(type) {
					case *plan.InSubquery, *expression.Equals, *expression.NullSafeEquals, *expression.GreaterThan,
						*expression.LessThan, *expression.GreaterThanOrEqual, *expression.LessThanOrEqual:
						if err := types.ErrIfMismatchedColumns(e.Children()[0].Type(), e.Children()[1].Type()); err != nil {
							panic(semError{err})
						}
					case *expression.InTuple, *expression.HashInTuple:
						t, ok := e.Children()[1].(expression.Tuple)
						if ok && len(t.Children()) == 1 {
							// A single element Tuple treats itself like the element it contains.
							if err := types.ErrIfMismatchedColumns(e.Children()[0].Type(), e.Children()[1].Type()); err != nil {
								panic(semError{err})
							}
						} else {
							if err := types.ErrIfMismatchedColumnsInTuple(e.Children()[0].Type(), e.Children()[1].Type()); err != nil {
								panic(semError{err})
							}
						}
					case *aggregation.Count, *aggregation.CountDistinct, *aggregation.JsonArray:
						if _, s := e.Children()[0].(*expression.Star); s {
							return false
						}
						for _, e := range e.Children() {
							nc := types.NumColumns(e.Type())
							if nc != 1 {
								panic(semError{sql.ErrInvalidOperandColumns.New(1, nc)})
							}
						}
					case expression.Tuple:
						// Tuple expressions can contain tuples...
					case *plan.ExistsSubquery:
						// Any number of columns are allowed.
					default:
						for _, e := range e.Children() {
							nc := types.NumColumns(e.Type())
							if nc != 1 {
								panic(semError{sql.ErrInvalidOperandColumns.New(1, nc)})
							}
						}
					}
					return true
				})
			}
		}
		return true
	})
}

func validateSubqueryColumns(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	// Then validate that every subquery has field indexes within the correct range
	// TODO: Why is this only for subqueries?

	// TODO: Currently disabled.
	if true {
		return
	}

	var outOfRangeIndexExpression sql.Expression
	var outOfRangeColumns int
	transform.InspectExpressionsWithNode(n, func(n sql.Node, e sql.Expression) bool {
		s, ok := e.(*plan.Subquery)
		if !ok {
			return true
		}

		outerScopeRowLen := len(scope.Schema()) + len(schemas(n.Children()))
		transform.Inspect(s.Query, func(n sql.Node) bool {
			if n == nil {
				return true
			}
			// TODO: the schema of the rows seen by children of
			// these nodes are not reflected in the schema
			// calculations here. This needs to be rationalized
			// across the analyzer.
			switch n := n.(type) {
			case *plan.JoinNode:
				return !n.Op.IsLookup()
			default:
			}
			if es, ok := n.(sql.Expressioner); ok {
				childSchemaLen := len(schemas(n.Children()))
				for _, e := range es.Expressions() {
					sql.Inspect(e, func(e sql.Expression) bool {
						if gf, ok := e.(*expression.GetField); ok {
							if gf.Index() >= outerScopeRowLen+childSchemaLen {
								outOfRangeIndexExpression = gf
								outOfRangeColumns = outerScopeRowLen + childSchemaLen
								panic(semError{analyzererrors.ErrSubqueryFieldIndex.New(outOfRangeIndexExpression, outOfRangeColumns)})
							}
						}
						return true
					})
				}
			}
			return true
		})
		return true
	})
}

// validateReadOnlyDatabase invalidates queries that attempt to write to ReadOnlyDatabases.
func validateReadOnlyDatabase(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	enforceReadOnly := scope.EnforcesReadOnly()

	// if a ReadOnlyDatabase is found, invalidate the query
	readOnlyDBSearch := func(node sql.Node) bool {
		if rt, ok := node.(*plan.ResolvedTable); ok {
			if ro, ok := rt.Database.(sql.ReadOnlyDatabase); ok {
				if ro.IsReadOnly() {
					panic(semError{analyzererrors.ErrReadOnlyDatabase.New(ro.Name())})
				} else if enforceReadOnly {
					panic(semError{sql.ErrProcedureCallAsOfReadOnly.New()})
				}
			}
		}
		return true
	}

	transform.Inspect(n, func(node sql.Node) bool {
		switch n := n.(type) {
		case *plan.DeleteFrom, *plan.Update, *plan.LockTables, *plan.UnlockTables:
			transform.Inspect(n, readOnlyDBSearch)
		case *plan.InsertInto:
			// ReadOnlyDatabase can be an insertion Source,
			// only inspect the Destination tree
			transform.Inspect(n.Destination, readOnlyDBSearch)
		case *plan.CreateTable:
			if ro, ok := n.Database().(sql.ReadOnlyDatabase); ok {
				if ro.IsReadOnly() {
					panic(semError{analyzererrors.ErrReadOnlyDatabase.New(ro.Name())})
				} else if enforceReadOnly {
					panic(semError{sql.ErrProcedureCallAsOfReadOnly.New()})
				}
			}
			// "CREATE TABLE ... LIKE ..." and
			// "CREATE TABLE ... AS ..."
			// can both use ReadOnlyDatabases as a source,
			// so don't descend here.
		default:
			// CreateTable is the only DDL node allowed
			// to contain a ReadOnlyDatabase
			if plan.IsDDLNode(n) {
				transform.Inspect(n, readOnlyDBSearch)
			}
		}
		return true
	})
}

// validateReadOnlyTransaction invalidates read only transactions that try to perform improper write operations.
func validateReadOnlyTransaction(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	t := ctx.GetTransaction()

	if t == nil {
		return
	}

	// If this is a normal read write transaction don't enforce read-only. Otherwise we must prevent an invalid query.
	if !t.IsReadOnly() && !scope.EnforcesReadOnly() {
		return
	}

	isTempTable := func(table sql.Table) bool {
		tt, isTempTable := table.(sql.TemporaryTable)
		if !isTempTable {
			panic(semError{sql.ErrReadOnlyTransaction.New()})
		}
		return tt.IsTemporary()
	}

	temporaryTableSearch := func(node sql.Node) bool {
		if rt, ok := node.(*plan.ResolvedTable); ok {
			if !isTempTable(rt.Table) {
				panic(semError{sql.ErrReadOnlyTransaction.New()})

			}
		}
		return false
	}

	transform.Inspect(n, func(node sql.Node) bool {
		switch n := n.(type) {
		case *plan.DeleteFrom, *plan.Update, *plan.UnlockTables:
			transform.Inspect(node, temporaryTableSearch)
		case *plan.InsertInto:
			transform.Inspect(n.Destination, temporaryTableSearch)
		case *plan.LockTables:
			// TODO: Technically we should allow for the locking of temporary tables but the LockTables implementation
			// needs substantial refactoring.
			panic(semError{sql.ErrReadOnlyTransaction.New()})
		case *plan.CreateTable:
			// MySQL explicitly blocks the creation of temporary tables in a read only transaction.
			if n.Temporary() == plan.IsTempTable {
				panic(semError{sql.ErrReadOnlyTransaction.New()})
			}
		default:
			// DDL statements have an implicit commits which makes them valid to be executed in READ ONLY transactions.
			if plan.IsDDLNode(n) {
				panic(semError{sql.ErrReadOnlyTransaction.New()})
			}
		}
		return true
	})
}

// validateAggregations returns an error if an Aggregation expression has been used in
// an invalid way, such as appearing outside of a GroupBy or Window node, or if an aggregate
// function is used with the implicit all-rows grouping and contains projected expressions with
// window aggregation functions that reference non-aggregated columns. Only GroupBy and Window
// nodes know how to evaluate Aggregation expressions.
//
// See https://github.com/dolthub/go-mysql-server/issues/542 for some queries
// that should be supported but that currently trigger this validation because
// aggregation expressions end up in the wrong place.
func validateAggregations(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	transform.Inspect(n, func(n sql.Node) bool {
		switch n := n.(type) {
		case *plan.GroupBy:
			checkForAggregationFunctions(n.GroupByExprs)
		case *plan.Window:
			checkForNonAggregatedColumnReferences(n)
		case sql.Expressioner:
			checkForAggregationFunctions(n.Expressions())
		default:
		}
		return true
	})
}

// checkForAggregationFunctions returns an ErrAggregationUnsupported error if any aggregation
// functions are found in the specified expressions.
func checkForAggregationFunctions(exprs []sql.Expression) {
	for _, e := range exprs {
		sql.Inspect(e, func(ie sql.Expression) bool {
			if _, ok := ie.(sql.Aggregation); ok {
				panic(semError{analyzererrors.ErrAggregationUnsupported.New(e.String())})
			}
			return true
		})
	}
}

// checkForNonAggregatedColumnReferences returns an ErrNonAggregatedColumnWithoutGroupBy error
// if an aggregate function with the implicit/all-rows grouping is mixed with aggregate window
// functions that reference a non-aggregated column.
// You cannot mix aggregations on the implicit/all-rows grouping with window aggregations.
func checkForNonAggregatedColumnReferences(w *plan.Window) {
	for _, expr := range w.ProjectedExprs() {
		if agg, ok := expr.(sql.Aggregation); ok {
			if agg.Window() == nil {
				index, gf := findFirstWindowAggregationColumnReference(w)
				if index > 0 {
					panic(semError{sql.ErrNonAggregatedColumnWithoutGroupBy.New(index+1, gf.String())})
				} else {
					// We should always have an index and GetField value to use, but just in case
					// something changes that, return a similar error message without those details.
					panic(semError{fmt.Errorf("in aggregated query without GROUP BY, expression in " +
						"SELECT list contains nonaggregated column; " +
						"this is incompatible with sql_mode=only_full_group_by")})
				}
			}
		}
	}
}

// findFirstWindowAggregationColumnReference returns the index and GetField expression for the
// first column reference in the first window aggregation function in the specified node's
// projection expressions. If no window aggregation function with a column reference is found,
// (-1, nil) is returned. This information is needed to populate an
// ErrNonAggregatedColumnWithoutGroupBy error.
func findFirstWindowAggregationColumnReference(w *plan.Window) (index int, gf *expression.GetField) {
	for index, expr := range w.ProjectedExprs() {
		var firstColumnRef *expression.GetField

		transform.InspectExpr(expr, func(e sql.Expression) bool {
			if windowAgg, ok := e.(sql.WindowAggregation); ok {
				transform.InspectExpr(windowAgg, func(e sql.Expression) bool {
					if gf, ok := e.(*expression.GetField); ok {
						firstColumnRef = gf
						return true
					}
					return false
				})
				return firstColumnRef != nil
			}
			return false
		})

		if firstColumnRef != nil {
			return index, firstColumnRef
		}
	}

	return -1, nil
}

func validateExprSem(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	transform.InspectExpressions(n, func(e sql.Expression) bool {
		validateSem(e)
		return true
	})
}

// validateSem is a way to add validation logic for
// specific expression types.
// todo(max): Refactor and consolidate validation so it can
// run before the rest of analysis. Add more expression types.
// Add node equivalent.
func validateSem(e sql.Expression) error {
	switch e := e.(type) {
	case *expression.And:
		logicalSem2(e.BinaryExpression)
	case *expression.Or:
		logicalSem2(e.BinaryExpression)
	default:
	}
	return nil
}

func logicalSem2(e expression.BinaryExpression) error {
	if lc := fds(e.Left); lc != 1 {
		panic(semError{sql.ErrInvalidOperandColumns.New(1, lc)})
	}
	if rc := fds(e.Right); rc != 1 {
		panic(semError{sql.ErrInvalidOperandColumns.New(1, rc)})
	}
	return nil
}

// fds counts the functional dependencies of an expression.
// todo(max): input/output fd's should be part of the expression
// interface.
func fds(e sql.Expression) int {
	switch e.(type) {
	case *expression.UnresolvedColumn:
		return 1
	case *expression.UnresolvedFunction:
		return 1
	default:
		return types.NumColumns(e.Type())
	}
}

// validatePrivileges verifies the given statement (node n) by checking that the calling user has the necessary privileges
// to execute it.
// TODO: add the remaining statements that interact with the grant tables
func validatePrivileges(ctx *sql.Context, a *Analyzer, n sql.Node, scope *Scope, sel RuleSelector) {
	mysqlDb := a.Catalog.MySQLDb
	switch n.(type) {
	case *plan.CreateUser, *plan.DropUser, *plan.RenameUser, *plan.CreateRole, *plan.DropRole,
		*plan.Grant, *plan.GrantRole, *plan.GrantProxy, *plan.Revoke, *plan.RevokeRole, *plan.RevokeAll, *plan.RevokeProxy:
		mysqlDb.Enabled = true
	}
	if !mysqlDb.Enabled {
		return
	}

	client := ctx.Session.Client()
	user := mysqlDb.GetUser(client.User, client.Address, false)
	if user == nil {
		panic(semError{mysql.NewSQLError(mysql.ERAccessDeniedError, mysql.SSAccessDeniedError, "Access denied for user '%v'", ctx.Session.Client().User)})
	}
	if plan.IsDualTable(getTable(n)) {
		return
	}
	if rt := getResolvedTable(n); rt != nil && rt.Database.Name() == sql.InformationSchemaDatabaseName {
		return
	}
	if !n.CheckPrivileges(ctx, a.Catalog.MySQLDb) {
		panic(semError{sql.ErrPrivilegeCheckFailed.New(user.UserHostToString("'"))})
	}
	return
}
