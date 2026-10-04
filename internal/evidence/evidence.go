package evidence

type Confidence string

const (
	Low    Confidence = "low"
	Medium Confidence = "medium"
	High   Confidence = "high"
)

// Evidence contains locations and summaries, never arbitrary source excerpts.
type Evidence struct {
	Kind       string     `json:"kind"`
	Message    string     `json:"message"`
	Source     string     `json:"source"`
	Confidence Confidence `json:"confidence"`
}
