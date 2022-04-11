package flypg

type Setting struct {
	Name           *string   `json:"name,omitempty"`
	Setting        *string   `json:"setting,omitempty"`
	Context        *string   `json:"context,omitempty"`
	VarType        *string   `json:"vartype,omitempty"`
	MinVal         *string   `json:"min_val,omitempty"`
	MaxVal         *string   `json:"max_val,omitempty"`
	EnumVals       []*string `json:"enumvals,omitempty"`
	Unit           *string   `json:"unit,omitempty"`
	ShortDesc      *string   `json:"short_desc,omitempty"`
	PendingChange  *string   `json:"pending_change,omitempty"`
	PendingRestart bool      `json:"pending_restart,omitempty"`
}

type Settings struct {
	Settings []Setting `json:"settings,omitempty"`
}
