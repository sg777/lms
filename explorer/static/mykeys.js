// My Keys functionality

async function loadMyKeys(silent = false) {
    if (!authToken) {
        return;
    }
    
    const container = document.getElementById('myKeysList');
    if (!container) return;
    
    if (!silent) {
        container.innerHTML = '<div class="loading">Loading your keys...</div>';
    }
    
    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/keys`);
        
        if (!response.ok) {
            if (response.status === 401) {
                // Token expired
                handleLogout();
                return;
            }
            throw new Error(`HTTP ${response.status}`);
        }
        
        const data = await response.json();
        
        if (!data.success || !data.keys || data.keys.length === 0) {
            container.innerHTML = '<div class="error">No keys found. Generate your first key to get started!</div>';
            updateSignKeySelect([]);
            updateVerifyKeySelect([]);
            loadWalletBalance();
            return;
        }
        
        // Load blockchain status for all keys (ignore non-JSON/HTML responses)
        let blockchainStatus = {};
        try {
            const statusResponse = await authenticatedFetch(`${API_BASE}/api/my/key/blockchain/status`);
            const ctype = statusResponse.headers.get('content-type') || '';
            if (statusResponse.ok && ctype.includes('application/json')) {
                const statusData = await statusResponse.json();
                if (statusData.success && statusData.settings) {
                    blockchainStatus = statusData.settings;
                }
            } else {
                // Skip if backend returned HTML/SPAs or non-JSON
                console.warn('Blockchain status response not JSON; status:', statusResponse.status);
            }
        } catch (err) {
            console.error('Failed to load blockchain status:', err);
        }
        
        // Display keys table
        let html = '<table><thead><tr><th>Key ID</th><th>Index</th><th>Parameters</th><th>Created</th><th>Blockchain</th><th>Actions</th></tr></thead><tbody>';
        
        data.keys.forEach(key => {
            const keyIdEscaped = escapeHtml(key.key_id).replace(/'/g, "\\'").replace(/"/g, '&quot;');
            const setting = blockchainStatus[key.key_id] || { enabled: false };
            const isEnabled = setting.enabled || false;
            html += `
                <tr>
                    <td><strong>${escapeHtml(key.key_id)}</strong></td>
                    <td>${key.index}</td>
                    <td class="hash-cell">${escapeHtml(key.params || 'N/A')}</td>
                    <td>${key.created ? new Date(key.created).toLocaleDateString() : 'N/A'}</td>
                    <td>
                        <label class="toggle-switch">
                            <input type="checkbox" ${isEnabled ? 'checked' : ''} 
                                   onchange="toggleBlockchain('${keyIdEscaped}', this.checked)"
                                   id="blockchain-toggle-${keyIdEscaped.replace(/[^a-zA-Z0-9]/g, '_')}">
                            <span class="toggle-slider"></span>
                        </label>
                    </td>
                    <td>
                        <button class="auth-btn" onclick="exportKey('${keyIdEscaped}')" style="margin-right: 5px; padding: 5px 10px; font-size: 0.85em;">📥 Export</button>
                        <button class="auth-btn" onclick="deleteKey('${keyIdEscaped}')" style="margin-right: 5px; padding: 5px 10px; font-size: 0.85em; background: #f87171;">🗑️ Delete</button>
                        <button class="auth-btn" onclick="if (typeof viewChain === 'function') { viewChain('${keyIdEscaped}'); } else { viewChainAfterSign('${keyIdEscaped}'); }" style="padding: 5px 10px; font-size: 0.85em;">🔗 View Chain</button>
                    </td>
                </tr>
            `;
        });
        
        html += '</tbody></table>';
        container.innerHTML = html;
        
        // Update sign key select and verify key select
        updateSignKeySelect(data.keys);
        updateVerifyKeySelect(data.keys);
        
        // Load wallet balance
        loadWalletBalance();
        
    } catch (error) {
        if (!silent) {
            container.innerHTML = `<div class="error">Error loading keys: ${error.message}</div>`;
        }
    }
}

// Load and display wallet balance
async function loadWalletBalance() {
    const balanceDisplay = document.getElementById('walletBalanceDisplay');
    if (!balanceDisplay) return;
    
    if (!authToken) {
        balanceDisplay.textContent = '💳 Please login';
        return;
    }
    
    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/wallet/total-balance`);
        const data = await response.json().catch(() => ({}));
        const balance = data.total_balance || 0;
        if (response.ok && data.success) {
            balanceDisplay.textContent = `💳 ${balance.toFixed(8)} CHIPS`;
            if (balance < 0.0001) {
                balanceDisplay.style.background = '#fee2e2';
                balanceDisplay.style.color = '#991b1b';
            } else {
                balanceDisplay.style.background = '#e0f2fe';
                balanceDisplay.style.color = '#0369a1';
            }
        } else {
            // On any error, show 0 and the error message
            balanceDisplay.textContent = `💳 0.00000000 CHIPS`;
            balanceDisplay.style.background = '#fee2e2';
            balanceDisplay.style.color = '#991b1b';
            if (data.error) {
                balanceDisplay.title = data.error;
            } else {
                balanceDisplay.title = `HTTP ${response.status}`;
            }
        }
    } catch (error) {
        balanceDisplay.textContent = '💳 0.00000000 CHIPS';
        balanceDisplay.style.background = '#fee2e2';
        balanceDisplay.style.color = '#991b1b';
        balanceDisplay.title = error.message;
        console.error('Failed to load wallet balance:', error);
    }
}

// Toggle blockchain for a key
async function toggleBlockchain(keyId, enable) {
    if (!authToken) {
        alert('Please login first');
        return;
    }
    
    const toggle = document.getElementById(`blockchain-toggle-${keyId.replace(/[^a-zA-Z0-9]/g, '_')}`);
    if (toggle) {
        toggle.disabled = true;
    }
    
    try {
        // Pre-check balance to avoid backend HTML responses
        let currentBalance = 0;
        try {
            const balResp = await authenticatedFetch(`${API_BASE}/api/my/wallet/total-balance`);
            const balData = await balResp.json().catch(() => ({}));
            if (balData && balData.total_balance !== undefined) {
                currentBalance = balData.total_balance || 0;
            }
        } catch (e) {
            // ignore, will rely on backend error
        }
        if (enable && currentBalance < 0.0001) {
            showCopyableError('❌ Not enough balance to enable blockchain. Please fund your wallet first.');
            if (toggle) {
                toggle.checked = false;
                toggle.disabled = false;
            }
            return;
        }

        const response = await authenticatedFetch(`${API_BASE}/api/my/key/blockchain/toggle`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                key_id: keyId,
                enable: enable
            })
        });
        
        const rawText = await response.text();
        let data = {};
        try {
            data = JSON.parse(rawText);
        } catch (e) {
            // Backend returned HTML or non-JSON
            throw new Error(rawText.slice(0, 200));
        }
        
        if (!response.ok || !data.success) {
            const errMsg = data.error || `HTTP ${response.status}`;
            throw new Error(errMsg);
        }
        
        if (data.success) {
            if (enable) {
                alert(`✅ Blockchain enabled for ${keyId}!\n\n${data.message || ''}\n\nTransaction ID: ${data.txid || 'N/A'}`);
            } else {
                alert(`✅ Blockchain disabled for ${keyId}`);
            }
            // Reload keys to refresh status
            await loadMyKeys();
            // Reload wallet balance (might have changed due to fees)
            await loadWalletBalance();
        } else {
            showCopyableError(`❌ Error: ${data.error || 'Failed to toggle blockchain'}`);
            // Revert toggle
            if (toggle) {
                toggle.checked = !enable;
                toggle.disabled = false;
            }
        }
    } catch (error) {
        showCopyableError(`❌ Error: ${error.message}\n\nIf your wallet balance is zero, fund it before enabling blockchain.`);
        // Revert toggle
        if (toggle) {
            toggle.checked = !enable;
            toggle.disabled = false;
        }
    }
}

// Show copyable error dialog
function showCopyableError(message) {
    // Remove any existing error dialog
    const existing = document.getElementById('errorDialog');
    if (existing) {
        existing.remove();
    }
    
    // Create dialog overlay
    const overlay = document.createElement('div');
    overlay.id = 'errorDialog';
    overlay.style.cssText = 'position: fixed; top: 0; left: 0; width: 100%; height: 100%; background: rgba(0,0,0,0.5); z-index: 10000; display: flex; align-items: center; justify-content: center;';
    
    // Create dialog box
    const dialog = document.createElement('div');
    dialog.style.cssText = 'background: white; border-radius: 8px; padding: 24px; max-width: 600px; max-height: 80vh; overflow: auto; box-shadow: 0 4px 6px rgba(0,0,0,0.1); position: relative;';
    
    // Create close button
    const closeBtn = document.createElement('button');
    closeBtn.textContent = '✕';
    closeBtn.style.cssText = 'position: absolute; top: 8px; right: 8px; background: none; border: none; font-size: 24px; cursor: pointer; color: #666; padding: 4px 8px;';
    closeBtn.onclick = () => overlay.remove();
    
    // Create title
    const title = document.createElement('div');
    title.textContent = 'Error';
    title.style.cssText = 'font-size: 20px; font-weight: bold; margin-bottom: 16px; color: #dc2626;';
    
    // Create textarea for copyable error message
    const textarea = document.createElement('textarea');
    textarea.value = message;
    textarea.readOnly = true;
    textarea.style.cssText = 'width: 100%; min-height: 150px; padding: 12px; border: 1px solid #ddd; border-radius: 4px; font-family: monospace; font-size: 13px; resize: vertical; white-space: pre-wrap; word-wrap: break-word;';
    textarea.onclick = (e) => e.target.select();
    
    // Create copy button
    const copyBtn = document.createElement('button');
    copyBtn.textContent = '📋 Copy Error Message';
    copyBtn.style.cssText = 'margin-top: 12px; padding: 8px 16px; background: #3b82f6; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 14px;';
    copyBtn.onclick = () => {
        textarea.select();
        document.execCommand('copy');
        copyBtn.textContent = '✓ Copied!';
        setTimeout(() => {
            copyBtn.textContent = '📋 Copy Error Message';
        }, 2000);
    };
    
    // Create OK button
    const okBtn = document.createElement('button');
    okBtn.textContent = 'OK';
    okBtn.style.cssText = 'margin-top: 12px; margin-left: 8px; padding: 8px 24px; background: #6b7280; color: white; border: none; border-radius: 4px; cursor: pointer; font-size: 14px;';
    okBtn.onclick = () => overlay.remove();
    
    // Assemble dialog
    const buttonContainer = document.createElement('div');
    buttonContainer.style.cssText = 'display: flex; justify-content: flex-end; margin-top: 12px;';
    buttonContainer.appendChild(copyBtn);
    buttonContainer.appendChild(okBtn);
    
    dialog.appendChild(closeBtn);
    dialog.appendChild(title);
    dialog.appendChild(textarea);
    dialog.appendChild(buttonContainer);
    overlay.appendChild(dialog);
    
    // Click outside to close
    overlay.onclick = (e) => {
        if (e.target === overlay) {
            overlay.remove();
        }
    };
    
    document.body.appendChild(overlay);
    
    // Auto-focus and select text
    setTimeout(() => {
        textarea.focus();
        textarea.select();
    }, 100);
}

function updateSignKeySelect(keys) {
    const select = document.getElementById('signKeySelect');
    if (!select) return;
    
    // Clear existing options (except first)
    while (select.options.length > 1) {
        select.remove(1);
    }
    
    // Add keys
    keys.forEach(key => {
        const option = document.createElement('option');
        option.value = key.key_id;
        option.textContent = `${key.key_id} (Index: ${key.index})`;
        select.appendChild(option);
    });
}

function updateVerifyKeySelect(keys) {
    const select = document.getElementById('verifyKeySelect');
    if (!select) return;
    
    // Clear existing options (except first default option)
    while (select.options.length > 1) {
        select.remove(1);
    }
    
    // Add keys
    keys.forEach(key => {
        const option = document.createElement('option');
        option.value = key.key_id;
        option.textContent = `${key.key_id} (Index: ${key.index})`;
        select.appendChild(option);
    });
}

async function handleGenerateKey() {
    if (!authToken) {
        alert('Please login first');
        return;
    }
    
    const btn = document.getElementById('generateKeyBtn');
    const originalText = btn.textContent;
    btn.textContent = 'Generating...';
    btn.disabled = true;
    
    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/generate`, {
            method: 'POST',
            body: JSON.stringify({
                key_id: '' // Let server generate
            })
        });
        
        if (!response.ok) {
            if (response.status === 401) {
                handleLogout();
                return;
            }
            const data = await response.json();
            throw new Error(data.error || 'Failed to generate key');
        }
        
        const data = await response.json();
        
        if (data.success) {
            // Reload keys list
            await loadMyKeys();
            alert(`Key generated successfully!\nKey ID: ${data.key_id}\nStarting Index: ${data.index}`);
        } else {
            throw new Error(data.error || 'Failed to generate key');
        }
    } catch (error) {
        showCopyableError('Error generating key: ' + error.message);
    } finally {
        btn.textContent = originalText;
        btn.disabled = false;
    }
}

// Copy to clipboard helper
function copyToClipboard(text) {
    navigator.clipboard.writeText(text).then(() => {
        alert('Signature copied to clipboard!');
    }).catch(err => {
        console.error('Failed to copy:', err);
        // Fallback: select text
        const textarea = document.createElement('textarea');
        textarea.value = text;
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
        alert('Signature copied to clipboard!');
    });
}

// Download signature as JSON file
function downloadSignature(keyId, signatureObj) {
    const jsonStr = JSON.stringify(signatureObj, null, 2);
    const blob = new Blob([jsonStr], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `signature_${keyId}_index${signatureObj.index || 'unknown'}.json`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}

// View chain after signing (switches to explorer tab)
function viewChainAfterSign(keyId) {
    // Switch to explorer tab first
    switchTab('explorer');
    
    // Wait for tab to be visible, then view chain
    setTimeout(() => {
        // Find the chain section and show it
        const chainSection = document.getElementById('chainSection');
        const chainView = document.getElementById('chainView');
        const chainTitle = document.getElementById('chainTitle');
        
        if (chainSection && chainView && chainTitle) {
            chainSection.style.display = 'block';
            chainTitle.textContent = `Chain: ${keyId}`;
            chainView.innerHTML = '<div class="loading">Loading chain...</div>';
            
            // Scroll to chain section
            chainSection.scrollIntoView({ behavior: 'smooth', block: 'start' });
            
            // Load the chain
            loadChainForKey(keyId);
        } else {
            // Fallback to original viewChain if elements not found
            if (typeof viewChain === 'function') {
                viewChain(keyId);
            }
        }
    }, 200);
}

// Load chain for a specific key (used after signing)
async function loadChainForKey(keyId) {
    const chainView = document.getElementById('chainView');
    if (!chainView) return;
    
    try {
        const response = await fetch(`${API_BASE}/api/chain/${encodeURIComponent(keyId)}`);
        const data = await response.json();

        if (!data.success || !data.chain) {
            chainView.innerHTML = '<div class="error">Failed to load chain</div>';
            return;
        }

        displayChain(data.chain, chainView);
    } catch (error) {
        chainView.innerHTML = `<div class="error">Error loading chain: ${error.message}</div>`;
    }
}

async function handleSignMessage() {
    if (!authToken) {
        alert('Please login first');
        return;
    }
    
    const keyID = document.getElementById('signKeySelect').value;
    const message = document.getElementById('signMessageInput').value.trim();
    const resultDiv = document.getElementById('signResult');
    
    if (!keyID) {
        resultDiv.innerHTML = '<div class="error-message">Please select a key</div>';
        resultDiv.style.display = 'block';
        return;
    }
    
    if (!message) {
        resultDiv.innerHTML = '<div class="error-message">Please enter a message</div>';
        resultDiv.style.display = 'block';
        return;
    }
    
    const btn = document.getElementById('signBtn');
    const originalText = btn.textContent;
    btn.textContent = 'Signing...';
    btn.disabled = true;
    resultDiv.style.display = 'none';
    
    try {
        const response = await authenticatedFetch(`${API_BASE}/api/my/sign`, {
            method: 'POST',
            body: JSON.stringify({
                key_id: keyID,
                message: message
            })
        });
        
        if (!response.ok) {
            if (response.status === 401) {
                handleLogout();
                return;
            }
            const data = await response.json();
            throw new Error(data.error || 'Failed to sign message');
        }
        
        const data = await response.json();
        
        if (data.success) {
            const sig = data.signature || {};
            const index = data.index !== undefined ? data.index : (sig.index !== undefined ? sig.index : 'N/A');
            const responseKeyId = data.key_id || keyID;
            
            // Format signature as JSON
            const signatureJSON = JSON.stringify(sig, null, 2);
            
            resultDiv.innerHTML = `
                <div class="success-message">
                    <strong>✅ Message signed successfully!</strong><br><br>
                    <strong>Key ID:</strong> ${escapeHtml(responseKeyId)}<br>
                    <strong>Index Used:</strong> ${index}<br><br>
                    <strong>Signature (structured JSON, copyable):</strong><br>
                    <div style="background: #f5f5f5; padding: 15px; border-radius: 6px; margin: 10px 0; word-break: break-all; font-family: 'Courier New', monospace; font-size: 0.85em; max-height: 300px; overflow-y: auto; border: 1px solid #ddd; cursor: text;" onclick="this.select(); document.execCommand('copy');" title="Click to select all, then copy">
                        ${escapeHtml(signatureJSON)}
                    </div>
                    <div style="margin-top: 10px;">
                        <button onclick="copyToClipboard('${escapeHtml(signatureJSON).replace(/'/g, "\\'").replace(/"/g, '&quot;').replace(/\n/g, '\\n')}')" class="auth-btn" style="margin-right: 10px;">📋 Copy Signature</button>
                        <button onclick="downloadSignature('${escapeHtml(responseKeyId).replace(/'/g, "\\'").replace(/"/g, '&quot;')}', ${escapeHtml(JSON.stringify(sig)).replace(/'/g, "\\'").replace(/"/g, '&quot;')})" class="auth-btn" style="margin-right: 10px;">💾 Download Signature</button>
                        <button class="auth-btn" onclick="viewChainAfterSign('${escapeHtml(responseKeyId).replace(/'/g, "\\'").replace(/"/g, '&quot;')}')">🔗 View Chain</button>
                    </div>
                </div>
            `;
            resultDiv.style.display = 'block';
            
            // Clear message input
            document.getElementById('signMessageInput').value = '';
            
            // Reload keys to update index
            await loadMyKeys(true);
            
            // Reload wallet balance (might have changed due to fees)
            await loadWalletBalance();
        } else {
            throw new Error(data.error || 'Failed to sign message');
        }
    } catch (error) {
        resultDiv.innerHTML = `<div class="error-message">Error: ${escapeHtml(error.message)}</div>`;
        resultDiv.style.display = 'block';
    } finally {
        btn.textContent = originalText;
        btn.disabled = false;
    }
}
