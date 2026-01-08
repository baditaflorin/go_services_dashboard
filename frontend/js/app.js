// === Services Dashboard App ===

class ServicesDashboard {
    constructor() {
        this.services = [];
        this.categories = [];
        this.currentCategory = '';
        this.currentStatus = '';
        this.searchQuery = '';
        this.viewMode = 'grid';

        this.init();
    }

    async init() {
        this.bindEvents();
        await this.fetchServices();
        await this.fetchStats();
        this.render();
        this.updateLastChecked();
        this.subscribeToEvents();

        // Auto-refresh countdown
        this.refreshInterval = 30;
        this.countdown = this.refreshInterval;
        this.startCountdown();

        // Expose for onclick handlers
        window.dashboard = this;
    }

    startCountdown() {
        setInterval(() => {
            this.countdown--;
            const countdownEl = document.getElementById('countdown');
            if (countdownEl) countdownEl.textContent = `${this.countdown}s`;

            if (this.countdown <= 0) {
                this.fetchServices();
                this.fetchStats();
                this.updateLastChecked();
                this.countdown = this.refreshInterval;
            }
        }, 1000);
    }

    updateLastChecked() {
        const el = document.getElementById('last-checked');
        if (el) {
            const now = new Date();
            el.textContent = `Last: ${now.toLocaleTimeString()}`;
        }
    }

    async refreshAll() {
        const btn = document.getElementById('refresh-btn');
        if (btn) {
            btn.textContent = '🔄 Refreshing...';
            btn.disabled = true;
        }

        try {
            await fetch('/api/refresh', { method: 'POST' });
            // Wait a moment for checks to complete
            setTimeout(async () => {
                await this.fetchServices();
                await this.fetchStats();
                this.render();
                this.updateLastChecked();
                this.countdown = this.refreshInterval;
            }, 2000);
        } catch (e) {
            console.error('Refresh failed:', e);
        } finally {
            if (btn) {
                btn.textContent = '🔄 Refresh All';
                btn.disabled = false;
            }
        }
    }

    bindEvents() {
        // Search
        document.getElementById('search').addEventListener('input', (e) => {
            this.searchQuery = e.target.value.toLowerCase();
            this.render();
        });

        // Category filters
        document.getElementById('category-list').addEventListener('click', (e) => {
            const category = e.target.closest('.category');
            if (category) {
                document.querySelectorAll('.category').forEach(c => c.classList.remove('active'));
                category.classList.add('active');
                this.currentCategory = category.dataset.category || '';
                this.render();
            }
        });

        // Status filters
        document.querySelectorAll('.filter').forEach(filter => {
            filter.addEventListener('click', () => {
                document.querySelectorAll('.filter').forEach(f => f.classList.remove('active'));
                filter.classList.add('active');
                this.currentStatus = filter.dataset.status || '';
                this.render();
            });
        });

        // View toggle
        document.querySelectorAll('.view-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                document.querySelectorAll('.view-btn').forEach(b => b.classList.remove('active'));
                btn.classList.add('active');
                this.viewMode = btn.dataset.view;
                this.updateViewMode();
            });
        });
    }

    async fetchServices() {
        try {
            const response = await fetch('/api/services');
            this.services = await response.json();
            this.buildCategories();
        } catch (error) {
            console.error('Failed to fetch services:', error);
        }
    }

    async fetchStats() {
        try {
            const response = await fetch('/api/stats');
            const stats = await response.json();

            document.getElementById('total-count').textContent = stats.total || 0;
            document.getElementById('healthy-count').textContent = stats.healthy || 0;
            document.getElementById('unhealthy-count').textContent = stats.unhealthy || 0;
            document.getElementById('uptime-percent').textContent =
                stats.healthy_percent ? `${stats.healthy_percent.toFixed(1)}%` : '0%';
            document.getElementById('all-count').textContent = stats.total || 0;
        } catch (error) {
            console.error('Failed to fetch stats:', error);
        }
    }

    buildCategories() {
        const counts = {};
        this.services.forEach(svc => {
            counts[svc.category] = (counts[svc.category] || 0) + 1;
        });

        this.categories = Object.entries(counts)
            .map(([name, count]) => ({ name, count }))
            .sort((a, b) => b.count - a.count);

        this.renderCategories();
    }

    renderCategories() {
        const list = document.getElementById('category-list');
        const allCount = this.services.length;

        list.innerHTML = `
            <li class="category ${!this.currentCategory ? 'active' : ''}" data-category="">
                <span class="category-name">All Services</span>
                <span class="category-count">${allCount}</span>
            </li>
            ${this.categories.map(cat => `
                <li class="category ${this.currentCategory === cat.name ? 'active' : ''}" data-category="${cat.name}">
                    <div class="category-header">
                        <span class="category-name">${this.formatCategoryName(cat.name)}</span>
                        <span class="category-count">${cat.count}</span>
                    </div>
                    <button class="test-all-btn" data-category="${cat.name}" onclick="event.stopPropagation(); window.dashboard.runCategoryTests('${cat.name}')">
                        Test All
                    </button>
                </li>
            `).join('')}
        `;
    }

    formatCategoryName(name) {
        const names = {
            'domains': '🌐 Domains',
            'security': '🔒 Security',
            'recon': '🔍 Recon',
            'infrastructure': '⚙️ Infrastructure',
            'web_analysis': '📊 Web Analysis'
        };
        return names[name] || name;
    }

    getFilteredServices() {
        return this.services.filter(svc => {
            // Category filter
            if (this.currentCategory && svc.category !== this.currentCategory) {
                return false;
            }

            // Status filter
            if (this.currentStatus && svc.status !== this.currentStatus) {
                return false;
            }

            // Search filter
            if (this.searchQuery) {
                const searchFields = [
                    svc.name,
                    svc.display_name,
                    svc.description,
                    ...(svc.tags || [])
                ].map(f => (f || '').toLowerCase());

                if (!searchFields.some(f => f.includes(this.searchQuery))) {
                    return false;
                }
            }

            return true;
        });
    }

    render() {
        const filtered = this.getFilteredServices();
        const grid = document.getElementById('services-grid');
        const countEl = document.getElementById('results-count');

        countEl.textContent = `${filtered.length} service${filtered.length !== 1 ? 's' : ''}`;

        if (filtered.length === 0) {
            grid.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">🔍</div>
                    <p>No services found matching your criteria.</p>
                </div>
            `;
            return;
        }

        grid.innerHTML = filtered.map(svc => this.renderServiceCard(svc)).join('');
        this.updateViewMode();
    }

    renderServiceCard(svc) {
        const statusClass = svc.status || 'unknown';
        const responseTimeClass = this.getResponseTimeClass(svc.response_ms);
        const healthHistoryHtml = this.renderHealthHistory(svc.health_history || []);
        const lastCheckedTime = svc.last_checked ? new Date(svc.last_checked).toLocaleTimeString() : '--';
        const testErrorTitle = svc.test_error ? `title="${svc.test_error}"` : '';

        return `
            <div id="service-${svc.id}" class="service-card ${statusClass}">
                <div class="card-header">
                    <h3 class="service-name">${svc.display_name}</h3>
                    <div class="status-indicator ${statusClass}">
                        <span class="dot"></span>
                        ${statusClass.charAt(0).toUpperCase() + statusClass.slice(1)}
                    </div>
                </div>
                
                <p class="service-description">${svc.description}</p>
                
                <div class="service-meta">
                    <span class="meta-tag category">${svc.category}</span>
                    <span class="meta-tag version">${svc.version ? 'v' + svc.version : 'Unknown'}</span>
                    <span class="meta-tag port">:${svc.port}</span>
                    ${(svc.tags || []).slice(0, 2).map(tag =>
            `<span class="meta-tag">${tag}</span>`
        ).join('')}
                </div>
                
                <div class="service-timing">
                    ${svc.response_ms > 0 ? `
                        <span class="response-time ${responseTimeClass}">${svc.response_ms}ms</span>
                    ` : ''}
                    <span class="last-check">Checked: ${lastCheckedTime}</span>
                </div>

                ${healthHistoryHtml}
                
                <div class="service-actions">
                    <div class="test-controls">
                        <a href="${svc.example_url}" target="_blank" class="test-link" title="${svc.example_url}">
                            <span class="link-icon">🔗</span> Test Link
                        </a>
                        <button onclick="window.dashboard.runTest('${svc.id}')" class="test-btn ${svc.test_status === 'passing' ? 'success' : (svc.test_status === 'failed' ? 'error' : '')}" ${testErrorTitle}>
                            Run Test ${svc.test_status === 'passing' ? '✓' : (svc.test_status === 'failed' ? '✗' : '')}
                        </button>
                    </div>
                    <div class="action-row">
                        <a href="${svc.health_url}" target="_blank" class="action-btn" title="Check Health Endpoint">Health</a>
                        <a href="${svc.repo_url}" target="_blank" class="action-btn primary" title="View Repository">Repo</a>
                    </div>
                </div>
            </div>
        `;
    }

    renderHealthHistory(history) {
        if (!history || history.length === 0) return '';
        const dots = history.map(status =>
            `<span class="history-dot ${status === 'healthy' ? 'healthy' : 'unhealthy'}" title="${status}"></span>`
        ).join('');
        return `<div class="health-history" title="Last ${history.length} health checks">${dots}</div>`;
    }

    async runTest(id) {
        const btn = document.querySelector(`button[onclick="window.dashboard.runTest('${id}')"]`);
        if (btn) {
            btn.textContent = "Testing...";
            btn.disabled = true;
        }

        try {
            const res = await fetch(`/api/test/${id}`, { method: 'POST' });
            const data = await res.json();
            if (btn) {
                if (data.test_status === 'passing') {
                    btn.textContent = 'Run Test ✓';
                    btn.classList.add('success');
                    btn.classList.remove('error');
                } else if (data.test_status === 'failed') {
                    btn.textContent = 'Run Test ✗';
                    btn.classList.add('error');
                    btn.classList.remove('success');
                } else {
                    btn.textContent = 'Run Test';
                }
            }
        } catch (e) {
            console.error("Test failed", e);
            if (btn) btn.textContent = 'Test Error';
        } finally {
            if (btn) btn.disabled = false;
        }
    }

    async runCategoryTests(category) {
        const btn = document.querySelector(`button[data-category="${category}"]`);
        if (btn) {
            btn.textContent = "Testing...";
            btn.disabled = true;
        }
        try {
            const res = await fetch(`/api/test-category/${category}`, { method: 'POST' });
            if (res.ok) {
                this.fetchServices();
            }
        } catch (e) {
            console.error("Category test failed", e);
        } finally {
            if (btn) {
                btn.textContent = `Test All ${category}`;
                btn.disabled = false;
            }
        }
    }

    getResponseTimeClass(ms) {
        if (!ms) return '';
        if (ms < 100) return 'fast';
        if (ms < 500) return 'medium';
        return 'slow';
    }

    updateViewMode() {
        const grid = document.getElementById('services-grid');
        grid.classList.toggle('list-view', this.viewMode === 'list');
    }

    subscribeToEvents() {
        const evtSource = new EventSource('/api/events');

        evtSource.onopen = () => {
            console.log('SSE Connected');
        };

        evtSource.onmessage = (event) => {
            if (!event.data) return;
            try {
                const data = JSON.parse(event.data);
                if (data.type === 'connected') return;
                this.handleUpdate(data);
            } catch (e) {
                console.error('SSE Parse Error', e);
            }
        };

        evtSource.onerror = (err) => {
            console.error('SSE Error:', err);
            // Browser attempts reconnection automatically
        };
    }

    handleUpdate(update) {
        const index = this.services.findIndex(s => s.id === update.id);
        if (index !== -1) {
            const oldStatus = this.services[index].status;

            // Merge updates
            this.services[index] = { ...this.services[index], ...update };

            // Re-render card if present
            const card = document.getElementById(`service-${update.id}`);
            if (card) {
                card.outerHTML = this.renderServiceCard(this.services[index]);
            }

            // Refresh stats if status changed
            if (update.status && oldStatus !== update.status) {
                this.fetchStats();
            }
        }
    }
}

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', () => {
    new ServicesDashboard();
});
