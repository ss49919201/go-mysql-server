// Code generated by "stringer -type=JoinType -linecomment"; DO NOT EDIT.

package plan

import (
	"strconv"
)

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[JoinTypeUnknown-0]
	_ = x[JoinTypeCross-1]
	_ = x[JoinTypeInner-2]
	_ = x[JoinTypeSemi-3]
	_ = x[JoinTypeAnti-4]
	_ = x[JoinTypeRightSemi-5]
	_ = x[JoinTypeLeftOuter-6]
	_ = x[JoinTypeFullOuter-7]
	_ = x[JoinTypeGroupBy-8]
	_ = x[JoinTypeRightOuter-9]
	_ = x[JoinTypeLookup-10]
	_ = x[JoinTypeLeftOuterLookup-11]
	_ = x[JoinTypeHash-12]
	_ = x[JoinTypeLeftOuterHash-13]
	_ = x[JoinTypeMerge-14]
	_ = x[JoinTypeLeftOuterMerge-15]
	_ = x[JoinTypeSemiHash-16]
	_ = x[JoinTypeAntiHash-17]
	_ = x[JoinTypeSemiLookup-18]
	_ = x[JoinTypeAntiLookup-19]
	_ = x[JoinTypeRightSemiLookup-20]
	_ = x[JoinTypeSemiMerge-21]
	_ = x[JoinTypeAntiMerge-22]
	_ = x[JoinTypeNatural-23]
}

const _JoinType_name = "UnknownJoinCrossJoinInnerJoinSemiJoinAntiJoinRightSemiJoinLeftOuterJoinFullOuterJoinGroupByJoinRightJoinLookupJoinLeftOuterLookupJoinHashJoinLeftOuterHashJoinMergeJoinLeftOuterMergeJoinSemiHashJoinAntiHashJoinSemiLookupJoinAntiLookupJoinRightSemiLookupJoinSemiMergeJoinAntiMergeJoinNaturalJoin"

var _JoinType_index = [...]uint16{0, 11, 20, 29, 37, 45, 58, 71, 84, 95, 104, 114, 133, 141, 158, 167, 185, 197, 209, 223, 237, 256, 269, 282, 293}

func (i JoinType) String() string {
	if i >= JoinType(len(_JoinType_index)-1) {
		return "JoinType(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _JoinType_name[_JoinType_index[i]:_JoinType_index[i+1]]
}
