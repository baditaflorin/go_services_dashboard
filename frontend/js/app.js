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

        // Auto-refresh every 30 seconds
        setInterval(() => {
            this.fetchServices();
            this.fetchStats();
        }, 30000);
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
                    <span class="category-name">${this.formatCategoryName(cat.name)}</span>
                    <span class="category-count">${cat.count}</span>
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

        return `
            <div class="service-card ${statusClass}">
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
                    <span class="meta-tag version">v${svc.version || 'unknown'}</span>
                    <span class="meta-tag port">:${svc.port}</span>
                    ${(svc.tags || []).slice(0, 2).map(tag =>
            `<span class="meta-tag">${tag}</span>`
        ).join('')}
                </div>
                
                ${svc.response_ms > 0 ? `
                    <div class="response-time ${responseTimeClass}">
                        Response: ${svc.response_ms}ms
                    </div>
                ` : ''}
                
                <div class="service-actions">
                    <a href="${svc.health_url}" target="_blank" class="action-btn">Health</a>
                    <a href="${svc.example_url}" target="_blank" class="action-btn">Test</a>
                    <a href="${svc.repo_url}" target="_blank" class="action-btn primary">Repo</a>
                </div>
            </div>
        `;
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
}

// Initialize on DOM ready
document.addEventListener('DOMContentLoaded', () => {
    new ServicesDashboard();
});
