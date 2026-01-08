// === State Management ===
// Centralized state with event listeners for updates

class State {
    constructor() {
        this.services = [];
        this.categories = [];
        this.stats = { total: 0, healthy: 0, unhealthy: 0, degraded: 0 };
        this.filters = {
            category: '',
            status: '',
            search: ''
        };
        this.viewMode = 'grid';
        this.listeners = [];
    }

    // Subscribe to state changes
    subscribe(callback) {
        this.listeners.push(callback);
        return () => {
            this.listeners = this.listeners.filter(cb => cb !== callback);
        };
    }

    // Notify all listeners of state change
    notify() {
        this.listeners.forEach(cb => cb(this));
    }

    // Update services and recompute stats
    setServices(services) {
        this.services = services || [];
        this.computeStats();
        this.buildCategories();
        this.notify();
    }

    // Compute stats from services
    computeStats() {
        this.stats = {
            total: this.services.length,
            healthy: this.services.filter(s => s.status === 'healthy').length,
            unhealthy: this.services.filter(s => s.status === 'unhealthy').length,
            degraded: this.services.filter(s => s.status === 'degraded').length
        };
        this.stats.healthyPercent = this.stats.total > 0
            ? (this.stats.healthy / this.stats.total * 100).toFixed(1)
            : 0;
    }

    // Build categories from services
    buildCategories() {
        const counts = {};
        this.services.forEach(svc => {
            counts[svc.category] = (counts[svc.category] || 0) + 1;
        });
        this.categories = Object.entries(counts)
            .map(([name, count]) => ({ name, count }))
            .sort((a, b) => b.count - a.count);
    }

    // Set filter
    setFilter(type, value) {
        this.filters[type] = value;
        this.notify();
    }

    // Set view mode
    setViewMode(mode) {
        this.viewMode = mode;
        this.notify();
    }

    // Get filtered services
    getFilteredServices() {
        return this.services.filter(svc => {
            if (this.filters.category && svc.category !== this.filters.category) return false;
            if (this.filters.status && svc.status !== this.filters.status) return false;
            if (this.filters.search) {
                const searchFields = [
                    svc.name, svc.display_name, svc.description, ...(svc.tags || [])
                ].map(f => (f || '').toLowerCase());
                if (!searchFields.some(f => f.includes(this.filters.search.toLowerCase()))) {
                    return false;
                }
            }
            return true;
        });
    }

    // Update single service test status
    updateServiceTest(id, testStatus, testError) {
        const svc = this.services.find(s => s.id === id);
        if (svc) {
            svc.test_status = testStatus;
            svc.test_error = testError;
            this.notify();
        }
    }
}

// Singleton instance
const state = new State();
export default state;
