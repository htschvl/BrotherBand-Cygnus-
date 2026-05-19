package handler

import "time"

// pingTimeout caps the Postgres ping in /readyz so a slow DB cannot
// stall the readiness response.
const pingTimeout = 1500 * time.Millisecond
