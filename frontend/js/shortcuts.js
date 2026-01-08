// === Keyboard Shortcuts ===
// R = Refresh, / = Focus search, Esc = Clear filter

class KeyboardShortcuts {
    constructor(dashboard) {
        this.dashboard = dashboard;
        this.init();
    }

    init() {
        document.addEventListener('keydown', (e) => {
            // Ignore if user is typing in an input
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
                if (e.key === 'Escape') {
                    e.target.blur();
                    e.target.value = '';
                    if (this.dashboard) {
                        this.dashboard.setFilter('search', '');
                    }
                }
                return;
            }

            switch (e.key.toLowerCase()) {
                case 'r':
                    e.preventDefault();
                    if (this.dashboard && typeof this.dashboard.refreshAll === 'function') {
                        this.dashboard.refreshAll();
                    }
                    break;
                case '/':
                    e.preventDefault();
                    const searchInput = document.getElementById('search');
                    if (searchInput) searchInput.focus();
                    break;
                case 'escape':
                    e.preventDefault();
                    if (this.dashboard) {
                        this.dashboard.setFilter('category', '');
                        this.dashboard.setFilter('status', '');
                        this.dashboard.setFilter('search', '');
                    }
                    break;
                case '1':
                    if (e.altKey && this.dashboard) {
                        this.dashboard.setFilter('status', 'healthy');
                    }
                    break;
                case '2':
                    if (e.altKey && this.dashboard) {
                        this.dashboard.setFilter('status', 'degraded');
                    }
                    break;
                case '3':
                    if (e.altKey && this.dashboard) {
                        this.dashboard.setFilter('status', 'unhealthy');
                    }
                    break;
            }
        });

        console.log('Keyboard shortcuts enabled: R=Refresh, /=Search, Esc=Clear, Alt+1/2/3=Filter');
    }
}

export default KeyboardShortcuts;
