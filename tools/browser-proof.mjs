import assert from 'node:assert/strict';

// Browser observations are supplied by the external MCP driver. Correlate its
// chat with the independent gateway trace before accepting that attestation.
export function validateBrowserProof(proof, evidence) {
  assert.equal(proof.driver, 'MCP Playwright');
  assert.equal(proof.sameOriginHealthStatus, 'ok');
  assert.equal(proof.healthVersion, evidence.revision.slice(0, 7));
  assert.equal(proof.catalogDiscovered, true);
  assert.equal(proof.chatStatus, 200);
  assert.equal(proof.chatModel, 'fixture/reasoner');
  assert.equal(proof.requestFetchSite, 'same-origin');
  assert.equal(proof.crossSiteNavigationStatusObserved, 403);
  assert(evidence.trace.some(r => r.gateway === 'stable' && r.tag === 'browser-same-origin' && r.model === proof.chatModel && r.status === proof.chatStatus), 'No matching browser chat reached the gateway');
  return { ...proof, gatewayCorrelationVerified: true,
    evidenceKind: 'External MCP browser attestation, correlated with the local gateway trace' };
}

export function browserProofPassed(proof, evidence) {
  if (proof?.gatewayCorrelationVerified !== true) return false;
  try { validateBrowserProof(proof, evidence); return true; } catch { return false; }
}
