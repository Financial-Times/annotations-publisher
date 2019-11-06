package annotations

type AnnotationsBody struct {
	Annotations []Annotation `json:"annotations"`
}

type Annotation struct {
	Predicate  string `json:"predicate"`
	ConceptID  string `json:"id"`
	APIURL     string `json:"apiUrl,omitempty"`
	Type       string `json:"type,omitempty"`
	PrefLabel  string `json:"prefLabel,omitempty"`
	IsFTAuthor bool   `json:"isFTAuthor,omitempty"`
}
