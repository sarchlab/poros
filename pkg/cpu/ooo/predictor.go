package ooo

import "github.com/sarchlab/poros/pkg/cpu/internal/branchpredictor"

// Aliases preserve the analytical backend's existing public configuration and state.
type PredictorSpec = branchpredictor.PredictorSpec
type PredictorState = branchpredictor.PredictorState
type predictor = branchpredictor.Predictor
