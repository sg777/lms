const API_BASE = window.location.origin;

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
    loadRecentCommits();
    loadStats();
    setupEventListeners();
    
    // Auto-refresh every 10 seconds
    setInterval(() => {
        loadRecentCommits();
        loadStats();
    }, 10000);
});

function setupEventListeners() {
    document.getElementById('searchBtn').addEventListener('click', handleSearch);
    document.getElementById('searchInput').addEventListener('keypress', (e) => {
        if (e.key === 'Enter') {
            handleSearch();
        }
    });
    document.getElementById('refreshBtn').addEventListener('click', () => {
        loadRecentCommits();
        loadStats();
    });
    document.getElementById('clearSearchBtn').addEventListener('click', clearSearch);
    document.getElementById('closeChainBtn').addEventListener('click', closeChain);
}

async function loadRecentCommits() {
    const container = document.getElementById('recentCommits');
    container.innerHTML = '<div class="loading">Loading recent commits...</div>';

    try {
        const response = await fetch(`${API_BASE}/api/recent?limit=50`);
        const data = await response.json();

        if (!data.success || !data.commits || data.commits.length === 0) {
            container.innerHTML = '<div class="error">No commits found</div>';
            return;
        }

        let html = '<table><thead><tr><th>Key ID</th><th>Index</th><th>Hash</th><th>Previous Hash</th></tr></thead><tbody>';
        
        data.commits.forEach(commit => {
            const hashShort = commit.hash ? truncateHash(commit.hash, 20) : '-';
            const prevHashShort = commit.previous_hash ? truncateHash(commit.previous_hash, 20) : '-';
            
            html += `
                <tr onclick="viewChain('${commit.key_id}')">
                    <td><strong>${escapeHtml(commit.key_id)}</strong></td>
                    <td>${commit.index}</td>
                    <td class="hash-cell" title="${commit.hash || ''}">${hashShort}</td>
                    <td class="hash-cell" title="${commit.previous_hash || ''}">${prevHashShort}</td>
                </tr>
            `;
        });

        html += '</tbody></table>';
        container.innerHTML = html;
    } catch (error) {
        container.innerHTML = `<div class="error">Error loading commits: ${error.message}</div>`;
    }
}

async function loadStats() {
    try {
        const response = await fetch(`${API_BASE}/api/stats`);
        const data = await response.json();

        if (!data.success) {
            return;
        }

        const stats = data.stats;
        document.getElementById('statTotalKeys').textContent = stats.total_keys || 0;
        document.getElementById('statTotalCommits').textContent = stats.total_commits || 0;
        document.getElementById('statValidChains').textContent = stats.valid_chains || 0;
        document.getElementById('statBrokenChains').textContent = stats.broken_chains || 0;
        
        if (stats.last_commit) {
            const date = new Date(stats.last_commit);
            document.getElementById('statLastUpdate').textContent = date.toLocaleString();
        } else {
            document.getElementById('statLastUpdate').textContent = 'Never';
        }
    } catch (error) {
        console.error('Error loading stats:', error);
    }
}

async function handleSearch() {
    const query = document.getElementById('searchInput').value.trim();
    if (!query) {
        return;
    }

    const resultsSection = document.getElementById('searchResultsSection');
    const resultsContainer = document.getElementById('searchResults');
    resultsSection.style.display = 'block';

    resultsContainer.innerHTML = '<div class="loading">Searching...</div>';

    try {
        const response = await fetch(`${API_BASE}/api/search?q=${encodeURIComponent(query)}`);
        const data = await response.json();

        if (data.type === 'not_found') {
            resultsContainer.innerHTML = `<div class="error">${escapeHtml(data.message)}</div>`;
            return;
        }

        if (data.type === 'key_id' && data.chain) {
            displayChain(data.chain, resultsContainer);
        } else if (data.type === 'hash' && data.entry) {
            displayEntry(data.entry, resultsContainer);
        } else {
            resultsContainer.innerHTML = `<div class="error">Unexpected response format</div>`;
        }
    } catch (error) {
        resultsContainer.innerHTML = `<div class="error">Search error: ${error.message}</div>`;
    }
}

async function viewChain(keyId) {
    const chainSection = document.getElementById('chainSection');
    const chainView = document.getElementById('chainView');
    const chainTitle = document.getElementById('chainTitle');
    
    chainSection.style.display = 'block';
    chainTitle.textContent = `Chain: ${keyId}`;
    chainView.innerHTML = '<div class="loading">Loading chain...</div>';

    // Scroll to chain section
    chainSection.scrollIntoView({ behavior: 'smooth' });

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

function displayChain(chain, container) {
    if (!chain.entries || chain.entries.length === 0) {
        container.innerHTML = '<div class="error">Chain is empty</div>';
        return;
    }

    let html = '';

    // Chain verification status
    const validBadge = chain.valid 
        ? '<span class="verification-badge valid">✓ VALID</span>'
        : '<span class="verification-badge broken">✗ BROKEN</span>';
    
    html += `<div class="${chain.valid ? 'success' : 'error'}" style="margin-bottom: 20px;">
        <strong>Chain Status:</strong> ${validBadge}
        ${chain.error ? `<br>Error: ${escapeHtml(chain.error)}</div>` : ''}
    </div>`;

    // Display each entry
    chain.entries.forEach((entry, index) => {
        const isGenesis = entry.is_genesis || index === 0;
        const isValid = entry.chain_valid !== false;
        const entryClass = isGenesis ? 'genesis' : (isValid ? 'valid' : 'broken');

        html += `
            <div class="chain-entry ${entryClass}">
                <div class="chain-entry-header">
                    <div class="chain-entry-title">
                        Entry ${index + 1} of ${chain.entries.length}
                        ${isGenesis ? '<span style="color: #667eea; margin-left: 10px;">(GENESIS)</span>' : ''}
                    </div>
                    <div class="chain-entry-index">Index: ${entry.index}</div>
                </div>
                <div class="chain-entry-details">
                    <div class="detail-item">
                        <div class="detail-label">Key ID</div>
                        <div class="detail-value">${escapeHtml(entry.key_id)}</div>
                    </div>
                    <div class="detail-item">
                        <div class="detail-label">Previous Hash</div>
                        <div class="detail-value full" title="${entry.previous_hash}">${entry.previous_hash || '-'}</div>
                    </div>
                    <div class="detail-item">
                        <div class="detail-label">Current Hash</div>
                        <div class="detail-value full" title="${entry.hash}">${entry.hash || '-'}</div>
                    </div>
                    <div class="detail-item">
                        <div class="detail-label">Signature</div>
                        <div class="detail-value full" title="${entry.signature}">${truncateHash(entry.signature, 40)}</div>
                    </div>
                </div>
                ${entry.chain_error ? `<div style="margin-top: 10px; color: #c33; font-size: 0.9em;">⚠ ${escapeHtml(entry.chain_error)}</div>` : ''}
            </div>
            ${index < chain.entries.length - 1 ? '<div class="chain-link"></div>' : ''}
        `;
    });

    container.innerHTML = html;
}

function displayEntry(entry, container) {
    container.innerHTML = `
        <div class="chain-entry ${entry.chain_valid ? 'valid' : 'broken'}">
            <div class="chain-entry-header">
                <div class="chain-entry-title">Entry Details</div>
                <div class="chain-entry-index">Index: ${entry.index}</div>
            </div>
            <div class="chain-entry-details">
                <div class="detail-item">
                    <div class="detail-label">Key ID</div>
                    <div class="detail-value">${escapeHtml(entry.key_id)}</div>
                </div>
                <div class="detail-item">
                    <div class="detail-label">Previous Hash</div>
                    <div class="detail-value full" title="${entry.previous_hash}">${entry.previous_hash || '-'}</div>
                </div>
                <div class="detail-item">
                    <div class="detail-label">Current Hash</div>
                    <div class="detail-value full" title="${entry.hash}">${entry.hash || '-'}</div>
                </div>
                <div class="detail-item">
                    <div class="detail-label">Signature</div>
                    <div class="detail-value full" title="${entry.signature}">${truncateHash(entry.signature, 40)}</div>
                </div>
            </div>
            ${entry.chain_error ? `<div style="margin-top: 10px; color: #c33;">⚠ ${escapeHtml(entry.chain_error)}</div>` : ''}
        </div>
        <div style="margin-top: 20px;">
            <button onclick="viewChain('${entry.key_id}')" class="search-btn">View Full Chain</button>
        </div>
    `;
}

function clearSearch() {
    document.getElementById('searchInput').value = '';
    document.getElementById('searchResultsSection').style.display = 'none';
}

function closeChain() {
    document.getElementById('chainSection').style.display = 'none';
}

function truncateHash(hash, length) {
    if (!hash) return '-';
    if (hash.length <= length) return hash;
    return hash.substring(0, length) + '...';
}

function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

