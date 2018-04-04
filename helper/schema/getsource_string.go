// Code generated by "stringer -type=getSource resource_data_get_source.go"; DO NOT EDIT.

package schema

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[getSourceState-1]
	_ = x[getSourceConfig-2]
	_ = x[getSourceDiff-4]
	_ = x[getSourceSet-8]
	_ = x[getSourceExact-16]
	_ = x[getSourceLevelMask-15]
}

const (
	_getSource_name_0 = "getSourceStategetSourceConfig"
	_getSource_name_1 = "getSourceDiff"
	_getSource_name_2 = "getSourceSet"
	_getSource_name_3 = "getSourceLevelMaskgetSourceExact"
)

var (
	_getSource_index_0 = [...]uint8{0, 14, 29}
	_getSource_index_1 = [...]uint8{0, 13}
	_getSource_index_2 = [...]uint8{0, 12}
	_getSource_index_3 = [...]uint8{0, 18, 32}
)

func (i getSource) String() string {
	switch {
	case 1 <= i && i <= 2:
		i -= 1
		return _getSource_name_0[_getSource_index_0[i]:_getSource_index_0[i+1]]
	case i == 4:
		return _getSource_name_1
	case i == 8:
		return _getSource_name_2
	case 15 <= i && i <= 16:
		i -= 15
		return _getSource_name_3[_getSource_index_3[i]:_getSource_index_3[i+1]]
	default:
		return "getSource(" + strconv.FormatInt(int64(i), 10) + ")"
	}
}
