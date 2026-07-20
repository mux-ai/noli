package extract

type RelationDraft struct {
	Predicate       string  `json:"predicate"`
	TargetReference string  `json:"target_reference"`
	Evidence        string  `json:"evidence,omitempty"`
	Confidence      float64 `json:"confidence"`
}
