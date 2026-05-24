package domain

// Prediction is one team's aggregated outcome across a Monte Carlo run.
//
// ChampionshipChance is a percentage in [0, 100]. AverageFinalPosition
// is the mean rank across simulations; MostLikelyFinalPosition is the
// modal rank. Predictions are computed on demand and are not persisted.
type Prediction struct {
	TeamID                  int64
	TeamName                string
	ChampionshipChance      float64
	AverageFinalPosition    float64
	MostLikelyFinalPosition int
}
