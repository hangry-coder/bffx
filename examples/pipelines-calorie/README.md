# pipelines-calorie

Minimal BFFX example for **AI ingestion pipelines** with the Open Food Facts catalog addon (no `batteries.nutrition`).

## Quick check

```bash
cd examples/pipelines-calorie
export GOWORK=off
go run ../../cmd/bffx sync   # or: bffx sync
go test ./... -short
```

Pipeline route: `POST /api/v1/meals/scan` (see `bffx/pipelines/meal_vision.yaml`).
