// detectors/index.ts — Detector registry.
//
// Add new detectors to this array to register them with the smart actions
// system. Each detector is run in order; all matching results are shown.

import type { Detector } from '../smartActions'
import { handoffDetector } from './handoffDetector'

export const detectors: ReadonlyArray<Detector> = [handoffDetector]
