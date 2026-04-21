(function () {
    function npsConfig() {
        window.nps = window.nps || {};
        return window.nps;
    }

    function safeLocalStorageGet(key) {
        try {
            return localStorage.getItem(key);
        } catch (e) {
            return null;
        }
    }

    function safeLocalStorageSet(key, value) {
        try {
            localStorage.setItem(key, value);
        } catch (e) {}
    }

    function normalizeThemeMode(mode) {
        if (mode === 'light' || mode === 'dark' || mode === 'auto') {
            return mode;
        }
        return 'auto';
    }

    function systemPrefersDark() {
        var mediaQuery = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
        return !!(mediaQuery && mediaQuery.matches);
    }

    function resolvedThemeMode(mode) {
        var normalized = normalizeThemeMode(mode);
        return normalized === 'dark' || (normalized === 'auto' && systemPrefersDark()) ? 'dark' : 'light';
    }

    function runtimeLangText(path, fallback) {
        var nps = npsConfig();
        if (typeof nps.langText === 'function') {
            return nps.langText(path, fallback);
        }
        return fallback;
    }

    function syncThemeToggle() {
        var theme = window.npsTheme;
        var mode = theme ? theme.getMode() : 'auto';
        var resolved = theme ? theme.getResolvedMode() : 'light';
        var buttons = document.querySelectorAll('#theme-toggle, [data-theme-toggle]');
        Array.prototype.forEach.call(buttons, function (button) {
            if (!button) {
                return;
            }
            var icon = button.querySelector('i');
            if (icon) {
                icon.classList.remove('fa-moon', 'fa-sun', 'fa-adjust');
                icon.classList.add(mode === 'auto' ? 'fa-adjust' : resolved === 'dark' ? 'fa-sun' : 'fa-moon');
            }
            var labels = {
                auto: runtimeLangText('theme-mode-auto'),
                dark: runtimeLangText('theme-mode-dark'),
                light: runtimeLangText('theme-mode-light')
            };
            button.setAttribute('title', labels[mode] || labels.auto);
            button.setAttribute('aria-label', labels[mode] || labels.auto);
            button.setAttribute('data-theme-mode', mode);
            button.setAttribute('data-theme-resolved', resolved);
        });
    }

    function applyThemeMode(mode) {
        var html = document.documentElement;
        var normalized = normalizeThemeMode(mode);
        var resolved = resolvedThemeMode(normalized);
        html.setAttribute('data-theme-mode', normalized);
        html.setAttribute('data-theme-resolved', resolved);
        if (resolved === 'dark') {
            html.setAttribute('theme', 'dark-mode');
        } else {
            html.removeAttribute('theme');
        }
        syncThemeToggle();
    }

    function persistedThemeMode() {
        return normalizeThemeMode(safeLocalStorageGet('nps-theme'));
    }

    function setThemeMode(mode) {
        var normalized = normalizeThemeMode(mode);
        safeLocalStorageSet('nps-theme', normalized);
        applyThemeMode(normalized);
        return normalized;
    }

    function toggleThemeMode() {
        var current = normalizeThemeMode(document.documentElement.getAttribute('data-theme-mode') || persistedThemeMode());
        var next = current === 'auto' ? 'dark' : current === 'dark' ? 'light' : 'auto';
        return setThemeMode(next);
    }

    window.toggleTheme = function () {
        return toggleThemeMode();
    };

    window.npsTheme = {
        key: 'nps-theme',
        getMode: function () {
            return normalizeThemeMode(document.documentElement.getAttribute('data-theme-mode') || persistedThemeMode());
        },
        getResolvedMode: function () {
            return resolvedThemeMode(this.getMode());
        },
        setMode: setThemeMode,
        toggle: toggleThemeMode,
        sync: function () {
            applyThemeMode(this.getMode());
        }
    };

    applyThemeMode(persistedThemeMode());

    var mediaQuery = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
    if (mediaQuery) {
        var onChange = function () {
            if (window.npsTheme && window.npsTheme.getMode() === 'auto') {
                applyThemeMode('auto');
            }
        };
        if (mediaQuery.addEventListener) {
            mediaQuery.addEventListener('change', onChange);
        } else if (mediaQuery.addListener) {
            mediaQuery.addListener(onChange);
        }
    }

    function joinBase(base, path) {
        base = (base || '').replace(/\/+$/, '');
        path = path || '';
        if (!path) {
            return base || '';
        }
        if (!base) {
            return path.charAt(0) === '/' ? path : '/' + path;
        }
        return base + (path.charAt(0) === '/' ? path : '/' + path);
    }

    function webBaseURL() {
        return (npsConfig().web_base_url || '').replace(/\/+$/, '');
    }

    function catalogKey(controller, action) {
        return String(controller || '').toLowerCase() + '/' + String(action || '').toLowerCase();
    }

    function catalogIndex(entries, cacheKey) {
        var nps = npsConfig();
        if (nps[cacheKey] && nps[cacheKey + 'Source'] === entries) {
            return nps[cacheKey];
        }
        var index = {};
        (entries || []).forEach(function (entry) {
            if (!entry || !entry.controller || !entry.action) {
                return;
            }
            index[catalogKey(entry.controller, entry.action)] = entry;
        });
        nps[cacheKey] = index;
        nps[cacheKey + 'Source'] = entries;
        return index;
    }

    function pageEntry(controller, action) {
        var nps = npsConfig();
        return catalogIndex(nps.page_catalog, '_pageCatalogIndex')[catalogKey(controller, action)] || null;
    }

    function actionEntry(controller, action) {
        var nps = npsConfig();
        return catalogIndex(nps.action_catalog, '_actionCatalogIndex')[catalogKey(controller, action)] || null;
    }

    function pageAvailable(controller, action) {
        return !!pageEntry(controller, action);
    }

    function isAdmin() {
        return !!npsConfig().is_admin;
    }

    function allowLocalProxy() {
        return featureEnabled('allow_local_proxy');
    }

    function allowUserLocal() {
        return featureEnabled('allow_user_local');
    }

    function featureEnabled(name) {
        name = String(name || '').trim();
        if (!name) {
            return false;
        }
        var aliases = {
            allow_user_vkey_login: 'allow_client_vkey_login'
        };
        var nps = npsConfig();
        if (!(nps.features && Object.prototype.hasOwnProperty.call(nps.features, name)) && aliases[name]) {
            name = aliases[name];
        }
        return !!(nps.features && Object.prototype.hasOwnProperty.call(nps.features, name) && nps.features[name]);
    }

    function localProxyAllowed() {
        return allowLocalProxy() && (isAdmin() || allowUserLocal());
    }

    function nodeActionAvailable(key) {
        key = String(key || '').toLowerCase().trim();
        if (!key) {
            return false;
        }
        return !!(npsConfig().node_action_available || {})[key];
    }

    function anyNodeActionAvailable(keys) {
        if (!Array.isArray(keys)) {
            return nodeActionAvailable(keys);
        }
        for (var i = 0; i < keys.length; i += 1) {
            if (nodeActionAvailable(keys[i])) {
                return true;
            }
        }
        return false;
    }

    function setAvailabilityVisible(element, visible) {
        if (!element) {
            return;
        }
        if (visible) {
            element.hidden = false;
            element.removeAttribute('aria-hidden');
            if (element.dataset && element.dataset.npsAvailabilityDisplay !== undefined) {
                element.style.display = element.dataset.npsAvailabilityDisplay;
                delete element.dataset.npsAvailabilityDisplay;
            } else {
                element.style.removeProperty('display');
            }
            return;
        }
        element.hidden = true;
        element.setAttribute('aria-hidden', 'true');
        if (element.dataset && element.dataset.npsAvailabilityDisplay === undefined) {
            element.dataset.npsAvailabilityDisplay = element.style.display || '';
        }
        element.style.display = 'none';
    }

    function featureList(raw) {
        return String(raw || '').split(',').map(function (value) {
            return value.trim();
        }).filter(Boolean);
    }

    function elementConditionalVisibility(element) {
        if (!element) {
            return true;
        }
        if (element.hasAttribute('data-nps-require-node-action') &&
            !nodeActionAvailable(element.getAttribute('data-nps-require-node-action'))) {
            return false;
        }
        if (element.hasAttribute('data-nps-require-any-node-actions') &&
            !anyNodeActionAvailable(featureList(element.getAttribute('data-nps-require-any-node-actions')))) {
            return false;
        }
        if (element.hasAttribute('data-nps-require-page-controller') &&
            element.hasAttribute('data-nps-require-page-action') &&
            !pageAvailable(
                element.getAttribute('data-nps-require-page-controller'),
                element.getAttribute('data-nps-require-page-action')
            )) {
            return false;
        }
        if (element.getAttribute('data-nps-require-admin') === 'true' && !isAdmin()) {
            return false;
        }
        if (element.getAttribute('data-nps-require-not-admin') === 'true' && isAdmin()) {
            return false;
        }
        if (element.hasAttribute('data-nps-require-feature') &&
            !featureEnabled(element.getAttribute('data-nps-require-feature'))) {
            return false;
        }
        if (element.hasAttribute('data-nps-require-any-features')) {
            var anyFeatures = featureList(element.getAttribute('data-nps-require-any-features'));
            if (!anyFeatures.some(featureEnabled)) {
                return false;
            }
        }
        if (element.getAttribute('data-nps-require-local-proxy') === 'true' && !localProxyAllowed()) {
            return false;
        }
        return true;
    }

    function syncConditionalVisibility(root) {
        var scope = root || document;
        if (!scope.querySelectorAll) {
            return;
        }
        var selectors = [
            '[data-nps-require-node-action]',
            '[data-nps-require-any-node-actions]',
            '[data-nps-require-page-controller][data-nps-require-page-action]',
            '[data-nps-require-admin]',
            '[data-nps-require-not-admin]',
            '[data-nps-require-feature]',
            '[data-nps-require-any-features]',
            '[data-nps-require-local-proxy]'
        ];
        scope.querySelectorAll(selectors.join(',')).forEach(function (element) {
            setAvailabilityVisible(element, elementConditionalVisibility(element));
        });
    }

    function actionPath(controller, action) {
        var entry = actionEntry(controller, action);
        if (entry) {
            if (entry.path) {
                return entry.path;
            }
            if (entry.protected) {
                return '';
            }
        }
        return joinBase(webBaseURL(), '/' + controller + '/' + action);
    }

    function nodeAPIPath(path) {
        var nps = npsConfig();
        return joinBase(nps.node_api_base || joinBase(webBaseURL(), '/api'), path);
    }

    function nodeRoute(name) {
        var nps = npsConfig();
        var key = String(name || '').toLowerCase().replace(/[\s-]+/g, '_');
        var aliases = {
            overview: 'node_overview',
            dashboard: 'node_dashboard',
            registration: 'node_registration',
            operations: 'node_operations',
            global: 'node_global',
            banlist: 'node_banlist',
            users: 'node_users',
            clients: 'node_clients',
            client_qr: 'node_clients_qr',
            clients_qr: 'node_clients_qr',
            clients_clear: 'node_clients_clear',
            tunnels: 'node_tunnels',
            hosts: 'node_hosts',
            config: 'node_config',
            status: 'node_status',
            changes: 'node_changes',
            callback_queue: 'node_callback_queue',
            callback_queue_replay: 'node_callback_queue_replay',
            callback_queue_clear: 'node_callback_queue_clear',
            usage: 'node_usage_snapshot',
            usage_snapshot: 'node_usage_snapshot',
            snapshot: 'node_usage_snapshot',
            batch: 'node_batch',
            traffic: 'node_traffic',
            kick: 'node_kick',
            sync: 'node_sync',
            ws: 'node_websocket',
            websocket: 'node_websocket'
        };
        if (aliases[key] && nps[aliases[key]]) {
            return nps[aliases[key]];
        }
        if (key) {
            return nodeAPIPath(key.replace(/_/g, '/'));
        }
        return nodeAPIPath('');
    }

    function nodeResourceName(resource) {
        var normalized = String(resource || '').toLowerCase().replace(/[\s_-]+/g, '');
        switch (normalized) {
            case 'user':
            case 'users':
                return 'users';
            case 'client':
            case 'clients':
                return 'clients';
            case 'tunnel':
            case 'tunnels':
                return 'tunnels';
            case 'host':
            case 'hosts':
                return 'hosts';
            default:
                return String(resource || '').toLowerCase();
        }
    }

    function nodeResourcePath(resource, id, subresource) {
        var name = nodeResourceName(resource);
        var path = nodeRoute(name);
        if (id !== undefined && id !== null && String(id) !== '') {
            path = joinBase(path, String(id));
        }
        if (subresource !== undefined && subresource !== null && String(subresource) !== '') {
            path = joinBase(path, String(subresource));
        }
        return path;
    }

    function nodeMutation(resource, action, id) {
        var name = nodeResourceName(resource);
        var normalized = String(action || '').toLowerCase();
        switch (normalized) {
            case 'add':
            case 'create':
                return {
                    method: 'POST',
                    url: nodeResourcePath(name)
                };
            case 'edit':
            case 'update':
            case 'save':
                return {
                    method: 'POST',
                    url: nodeResourcePath(name, id, 'actions/update')
                };
            case 'delete':
            case 'del':
                return {
                    method: 'POST',
                    url: nodeResourcePath(name, id, 'actions/delete')
                };
            case 'status':
            case 'changestatus':
                return {
                    method: 'POST',
                    url: nodeResourcePath(name, id, 'actions/status')
                };
            case 'clear':
                return {
                    method: 'POST',
                    url: nodeResourcePath(name, id, 'actions/clear')
                };
            case 'start':
                return {
                    method: 'POST',
                    url: nodeResourcePath(name, id, 'actions/start')
                };
            case 'stop':
                return {
                    method: 'POST',
                    url: nodeResourcePath(name, id, 'actions/stop')
                };
            case 'ping':
                return {
                    method: 'POST',
                    url: nodeResourcePath(name, id, 'actions/ping')
                };
            default:
                return {
                    method: 'POST',
                    url: nodeResourcePath(name, id, 'actions/' + normalized)
                };
        }
    }

    function encodeQuery(params) {
        if (!params) {
            return '';
        }
        var query = new URLSearchParams();
        Object.keys(params).forEach(function (key) {
            var value = params[key];
            if (value === null || value === undefined || value === '') {
                return;
            }
            query.append(key, String(value));
        });
        var encoded = query.toString();
        return encoded ? ('?' + encoded) : '';
    }

    function pagePath(controller, action, params) {
        var entry = pageEntry(controller, action);
        var path = joinBase(webBaseURL(), '/' + controller + '/' + action);
        if (entry) {
            path = entry.direct_path || entry.page_path || entry.render_path || path;
        }
        return path + encodeQuery(params);
    }

    function appShellPath(path) {
        return joinBase(npsConfig().management_shell || joinBase(webBaseURL(), '/app'), path || '');
    }

    function pageParamsFromAttribute(raw) {
        if (!raw) {
            return null;
        }
        var params = {};
        var query = new URLSearchParams(String(raw));
        query.forEach(function (value, key) {
            if (value === '') {
                return;
            }
            params[key] = value;
        });
        return Object.keys(params).length ? params : null;
    }

    function enhancePageLinks(root) {
        var scope = root || document;
        if (!scope.querySelectorAll) {
            return;
        }
        scope.querySelectorAll('a[data-nps-page-controller][data-nps-page-action]').forEach(function (link) {
            var controller = link.getAttribute('data-nps-page-controller');
            var action = link.getAttribute('data-nps-page-action');
            if (!controller || !action) {
                return;
            }
            link.href = pagePath(controller, action, pageParamsFromAttribute(link.getAttribute('data-nps-page-query')));
        });
    }

    function mergeConfig(patch) {
        var nps = npsConfig();
        patch = patch || {};
        if (Object.prototype.hasOwnProperty.call(patch, 'page_catalog')) {
            delete nps._pageCatalogIndex;
            delete nps._pageCatalogIndexSource;
        }
        if (Object.prototype.hasOwnProperty.call(patch, 'action_catalog')) {
            delete nps._actionCatalogIndex;
            delete nps._actionCatalogIndexSource;
        }
        Object.keys(patch).forEach(function (key) {
            nps[key] = patch[key];
        });
        return nps;
    }

    function normalizedCurrentPath() {
        var base = webBaseURL();
        var pathname = window.location && window.location.pathname ? String(window.location.pathname) : '/';
        if (base && pathname.indexOf(base) === 0) {
            pathname = pathname.substring(base.length) || '/';
        }
        if (!pathname) {
            pathname = '/';
        }
        return pathname.charAt(0) === '/' ? pathname : '/' + pathname;
    }

    function normalizedNavigationKey() {
        var pathname = normalizedCurrentPath();
        if (pathname === '/' || pathname === '/index' || pathname === '/index/index') {
            return 'index/index';
        }
        if (pathname.indexOf('/client/') === 0) {
            return 'client/list';
        }
        if (pathname.indexOf('/user/') === 0) {
            return 'user/list';
        }
        if (pathname === '/global/index') {
            return 'global/index';
        }
        if (pathname === '/global/banlist') {
            return 'global/banlist';
        }
        if (pathname === '/global/callbackqueue') {
            return 'global/callbackqueue';
        }
        if (pathname === '/index/hostlist' || pathname === '/index/addhost' || pathname === '/index/edithost') {
            return 'index/hostlist';
        }
        if (pathname === '/index/add' || pathname === '/index/edit') {
            var type = String(window.nps.pageParam('type') || '').toLowerCase();
            if (type) {
                return 'index/' + type;
            }
        }
        if (pathname.indexOf('/index/') === 0) {
            var parts = pathname.replace(/^\/+/, '').split('/');
            if (parts.length >= 2) {
                return catalogKey(parts[0], parts[1]);
            }
        }
        return '';
    }

    function navigationSpecs() {
        return [
            { controller: 'index', action: 'index', icon: 'fa-tachometer-alt', langtag: 'word-dashboard' },
            { controller: 'client', action: 'list', icon: 'fa-desktop', langtag: 'word-client' },
            { controller: 'user', action: 'list', icon: 'fa-users', langtag: 'word-user' },
            { controller: 'index', action: 'hostlist', icon: 'fa-globe', langtag: 'scheme-host' },
            { controller: 'index', action: 'tcp', icon: 'fa-retweet', langtag: 'scheme-tcp' },
            { controller: 'index', action: 'udp', icon: 'fa-random', langtag: 'scheme-udp' },
            { controller: 'index', action: 'mix', icon: 'fa-layer-group', langtag: 'scheme-mixproxy' },
            { controller: 'index', action: 'secret', icon: 'fa-low-vision', langtag: 'scheme-secret' },
            { controller: 'index', action: 'p2p', icon: 'fa-exchange-alt', langtag: 'scheme-p2p' },
            { controller: 'index', action: 'file', icon: 'fa-briefcase', langtag: 'scheme-file' },
            { controller: 'global', action: 'index', icon: 'fa-cog', langtag: 'word-globalparam' },
            { controller: 'global', action: 'banlist', icon: 'fa-ban', langtag: 'word-banlist' },
            { controller: 'global', action: 'callbackqueue', icon: 'fa-paper-plane', langtag: 'word-callbackqueue' },
            { controller: 'index', action: 'help', icon: 'fa-lightbulb', langtag: 'word-help', external: 'https://d-jy.net/docs/nps/' }
        ];
    }

    function navigationHref(spec) {
        if (!spec) {
            return '#';
        }
        if (spec.external) {
            return spec.external;
        }
        return pagePath(spec.controller, spec.action);
    }

    function buildNavigationItem(spec, activeKey) {
        var key = catalogKey(spec.controller, spec.action);
        var li = document.createElement('li');
        li.className = key === activeKey ? 'active' : '';
        li.setAttribute('data-nps-nav-key', key);
        var link = document.createElement('a');
        link.href = navigationHref(spec);
        link.className = 'd-flex align-items-center';
        if (!spec.external) {
            link.setAttribute('data-nps-page-controller', spec.controller);
            link.setAttribute('data-nps-page-action', spec.action);
        } else {
            link.target = '_blank';
            link.rel = 'noopener noreferrer';
        }
        var icon = document.createElement('i');
        icon.className = 'fa ' + spec.icon + ' fa-lg fa-fw';
        link.appendChild(icon);
        var span = document.createElement('span');
        span.className = 'nav-label';
        span.setAttribute('langtag', spec.langtag);
        link.appendChild(span);
        li.appendChild(link);
        return li;
    }

    function syncNavigation(root) {
        var scope = root || document;
        var nav = scope.querySelector ? scope.querySelector('#side-menu[data-nps-nav]') : null;
        if (!nav) {
            return;
        }
        var header = nav.querySelector('.nav-header');
        while (nav.lastChild && nav.lastChild !== header) {
            nav.removeChild(nav.lastChild);
        }
        var activeKey = normalizedNavigationKey();
        navigationSpecs().forEach(function (spec) {
            if (!spec) {
                return;
            }
            if (!spec.external && !pageEntry(spec.controller, spec.action)) {
                return;
            }
            nav.appendChild(buildNavigationItem(spec, activeKey));
        });
    }

    var nps = npsConfig();
    nps.joinBase = joinBase;
    nps.actionPath = actionPath;
    nps.nodeAPIPath = nodeAPIPath;
    nps.nodeRoute = nodeRoute;
    nps.nodeResourcePath = nodeResourcePath;
    nps.nodeMutation = nodeMutation;
    nps.pagePath = pagePath;
    nps.appShellPath = appShellPath;
    nps.pageEntry = pageEntry;
    nps.actionEntry = actionEntry;
    nps.pageAvailable = pageAvailable;
    nps.isAdmin = isAdmin;
    nps.allowLocalProxy = allowLocalProxy;
    nps.allowUserLocal = allowUserLocal;
    nps.localProxyAllowed = localProxyAllowed;
    nps.featureEnabled = featureEnabled;
    nps.nodeActionAvailable = nodeActionAvailable;
    nps.anyNodeActionAvailable = anyNodeActionAvailable;
    nps.hasPage = function (controller, action) {
        return !!pageEntry(controller, action);
    };
    nps.hasAction = function (controller, action) {
        return !!actionEntry(controller, action);
    };
    nps.enhancePageLinks = enhancePageLinks;
    nps.syncNavigation = syncNavigation;
    nps.syncConditionalVisibility = syncConditionalVisibility;
    nps.mergeConfig = mergeConfig;
    nps.syncThemeToggle = syncThemeToggle;
    nps.setThemeMode = function (mode) {
        if (window.npsTheme) {
            return window.npsTheme.setMode(mode);
        }
        return mode;
    };
    nps.toggleTheme = function () {
        if (window.npsTheme) {
            return window.npsTheme.toggle();
        }
        return 'auto';
    };

})();
