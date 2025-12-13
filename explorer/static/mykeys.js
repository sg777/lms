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
            html += `
                <tr>
                    <td><strong>${escapeHtml(key.key_id)}</strong></td>
                    <td>${key.index}</td>
                    <td class="hash-cell">${escapeHtml(key.params || 'N/A')}</td>
                    <td>${new Date(key.created).toLocaleDateString()}</td>
                    <td><button class="auth-btn" onclick="viewChain('${escapeHtml(key.key_id).replace(/'/g, "\\'")}')">View Chain</button></td>
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
            resultDiv.innerHTML = `
                <div class="success-message">
                    <strong>Message signed successfully!</strong><br>
                    Key ID: ${escapeHtml(data.key_id)}<br>
                    Index: ${data.index}<br>
                    Signature: ${escapeHtml(data.signature.substring(0, 50))}...<br>
                    <button class="auth-btn" onclick="viewChain('${escapeHtml(keyID).replace(/'/g, "\\'")}')" style="margin-top: 10px;">View Chain</button>
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

