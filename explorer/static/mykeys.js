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
            return;
        }
        
        // Display keys table
        let html = '<table><thead><tr><th>Key ID</th><th>Index</th><th>Parameters</th><th>Created</th><th>Actions</th></tr></thead><tbody>';
        
        data.keys.forEach(key => {
            const keyIdEscaped = escapeHtml(key.key_id).replace(/'/g, "\\'").replace(/"/g, '&quot;');
            html += `
                <tr>
                    <td><strong>${escapeHtml(key.key_id)}</strong></td>
                    <td>${key.index}</td>
                    <td class="hash-cell">${escapeHtml(key.params || 'N/A')}</td>
                    <td>${key.created ? new Date(key.created).toLocaleDateString() : 'N/A'}</td>
                    <td>
                        <button class="auth-btn" onclick="exportKey('${keyIdEscaped}')" style="margin-right: 5px; padding: 5px 10px; font-size: 0.85em;">📥 Export</button>
                        <button class="auth-btn" onclick="deleteKey('${keyIdEscaped}')" style="margin-right: 5px; padding: 5px 10px; font-size: 0.85em; background: #f87171;">🗑️ Delete</button>
                        <button class="auth-btn" onclick="viewChain('${keyIdEscaped}')" style="padding: 5px 10px; font-size: 0.85em;">🔗 View Chain</button>
                    </td>
                </tr>
            `;
        });
        
        html += '</tbody></table>';
        container.innerHTML = html;
        
        // Update sign key select
        updateSignKeySelect(data.keys);
        
    } catch (error) {
        if (!silent) {
            container.innerHTML = `<div class="error">Error loading keys: ${error.message}</div>`;
        }
    }
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
        alert('Error generating key: ' + error.message);
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
            const signature = data.signature || '';
            const index = data.index !== undefined ? data.index : 'N/A';
            
            resultDiv.innerHTML = `
                <div class="success-message">
                    <strong>Message signed successfully!</strong><br><br>
                    <strong>Key ID:</strong> ${escapeHtml(data.key_id || keyID)}<br>
                    <strong>Index Used:</strong> ${index}<br><br>
                    <strong>Signature (full):</strong><br>
                    <div style="background: #f5f5f5; padding: 10px; border-radius: 4px; margin: 10px 0; word-break: break-all; font-family: monospace; font-size: 0.9em; max-height: 200px; overflow-y: auto;">
                        ${escapeHtml(signature)}
                    </div>
                    <button onclick="copyToClipboard('${escapeHtml(signature).replace(/'/g, "\\'")}')" class="auth-btn" style="margin-right: 10px;">📋 Copy Signature</button>
                    <button class="auth-btn" onclick="viewChainAfterSign('${escapeHtml(data.key_id || keyID).replace(/'/g, "\\'")}')">View Chain</button>
                </div>
            `;
            resultDiv.style.display = 'block';
            
            // Clear message input
            document.getElementById('signMessageInput').value = '';
            
            // Reload keys to update index
            await loadMyKeys(true);
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

