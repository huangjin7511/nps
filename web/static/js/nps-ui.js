(function ($) {
    function currentLang() {
        return window.nps.currentLang();
    }

    function isZhLanguage() {
        return window.nps.isZhLanguage();
    }

    function uiLangText(path, fallback) {
        if (window.nps && typeof window.nps.langText === 'function') {
            return window.nps.langText(path, fallback);
        }
        return fallback || '';
    }

    function uiLangFormat(path, replacements, fallback) {
        if (window.nps && typeof window.nps.langFormat === 'function') {
            return window.nps.langFormat(path, replacements, fallback);
        }
        return fallback || '';
    }

    function applyReadOnlyForm(formSelector, options) {
        var $form = $(formSelector);
        if (!$form.length) {
            return;
        }
        var settings = $.extend({
            message: '',
            alertClass: 'alert alert-info m-b-md nps-readonly-banner'
        }, options || {});
        $form.attr('data-nps-readonly', 'true');
        var $banner = $form.prevAll('.nps-readonly-banner').first();
        if (settings.message) {
            if ($banner.length) {
                $banner.attr('class', settings.alertClass).text(settings.message);
            } else {
                $('<div/>').addClass(settings.alertClass).text(settings.message).insertBefore($form);
            }
        }
        $form.find('input, textarea, select').each(function () {
            var $field = $(this);
            var type = String($field.attr('type') || '').toLowerCase();
            if (type === 'hidden') {
                return;
            }
            if ($field.is('select') || type === 'checkbox' || type === 'radio') {
                $field.prop('disabled', true);
                return;
            }
            $field.prop('readonly', true).addClass('nps-readonly');
        });
        $form.find('.selectpicker').each(function () {
            var $select = $(this);
            $select.prop('disabled', true);
            if (typeof $select.selectpicker === 'function') {
                try {
                    $select.selectpicker('refresh');
                } catch (e) {}
            }
        });
    }

    function isReadOnlyForm(formSelector) {
        var $form = $(formSelector);
        return !!($form.length && $form.attr('data-nps-readonly') === 'true');
    }

    function defaultEmptySelectText() {
        return uiLangText('info-unassigned-platform-managed');
    }

    function initRemoteSelectPicker(selector, options) {
        var $select = $(selector);
        if (!$select.length) {
            return;
        }
        options = $.extend({
            method: 'GET',
            url: '',
            data: { order: 'asc', offset: 0, limit: 0 },
            selectedValue: '',
            includeEmpty: false,
            emptyText: '',
            searchField: 'search',
            rows: function (response) {
                if (response && response.code === 1 && response.data && $.isArray(response.data.items)) {
                    return response.data.items;
                }
                return response && $.isArray(response.rows) ? response.rows : [];
            },
            mapItem: function (item, selectedValue) {
                return {
                    value: item.Id,
                    text: String(item.Id),
                    selected: String(item.Id) === String(selectedValue)
                };
            },
            onLoaded: null
        }, options || {});
        var selectedValue = options.selectedValue;
        var buildItems = function (response) {
            var rows = options.rows(response) || [];
            var results = [];
            if (options.includeEmpty) {
                results.push({
                    value: '',
                    text: options.emptyText || defaultEmptySelectText(),
                    selected: selectedValue === '' || selectedValue === null || selectedValue === undefined
                });
            }
            $.each(rows, function (_, item) {
                results.push(options.mapItem(item, selectedValue));
            });
            return results;
        };
        var request = function (callback, searchTerm) {
            var requestData = $.extend({}, options.data || {});
            if (searchTerm) {
                requestData[options.searchField] = searchTerm;
            }
            $.ajax({
                method: options.method,
                url: options.url,
                dataType: 'json',
                data: requestData,
                success: function (response) {
                    callback(buildItems(response));
                    if (typeof options.onLoaded === 'function') {
                        window.setTimeout(options.onLoaded, 0);
                    }
                },
                error: function () {
                    callback(buildItems(null));
                    if (typeof options.onLoaded === 'function') {
                        window.setTimeout(options.onLoaded, 0);
                    }
                }
            });
        };
        try {
            if ($select.data('selectpicker')) {
                $select.selectpicker('destroy');
            }
        } catch (e) {}
        $select.selectpicker({
            liveSearch: true,
            source: {
                data: function (callback) {
                    request(callback, '');
                },
                search: function (callback, page, searchTerm) {
                    request(callback, searchTerm);
                }
            }
        });
    }

    function setGroupVisibility(selector, visible, displayValue) {
        var $el = $(selector);
        if (!$el.length) {
            return;
        }
        $el.css('display', visible ? (displayValue || 'block') : 'none');
    }

    function pageParam(name) {
        if (!name || typeof URLSearchParams === 'undefined') {
            return '';
        }
        return new URLSearchParams(window.location.search).get(String(name)) || '';
    }

    function pageIntParam(name) {
        var value = parseInt(pageParam(name), 10);
        return isNaN(value) ? 0 : value;
    }

    function setSelectValue(target, value) {
        var $target = $(target);
        if (!$target.length) {
            return;
        }
        var normalized = value === null || value === undefined ? '' : String(value);
        $target.val(normalized);
        if ($target.hasClass('selectpicker') && typeof $target.selectpicker === 'function') {
            try {
                $target.selectpicker('refresh');
            } catch (e) {}
        }
    }

    function setFormValue(formSelector, name, value) {
        var $fields = $(formSelector).find('[name="' + name + '"]');
        if (!$fields.length) {
            return;
        }
        $fields.each(function () {
            var $field = $(this);
            var type = String($field.attr('type') || '').toLowerCase();
            if ($field.is('select')) {
                setSelectValue($field, value);
                return;
            }
            if (type === 'checkbox') {
                var checked = value === true || value === 1 || value === '1' || value === 'true';
                $field.prop('checked', checked);
                return;
            }
            $field.val(value === null || value === undefined ? '' : value);
        });
    }

    function setTextContent(target, value) {
        var $target = $(target);
        if (!$target.length) {
            return;
        }
        $target.text(value === null || value === undefined ? '' : String(value));
    }

    function resourceItemEnvelopeData(response) {
        var data = resourceEnvelopeData(response);
        if (data && data.item && typeof data.item === 'object') {
            return data.item;
        }
        return null;
    }

    function fetchNodeData(url, method, payload) {
        return $.ajax({
            url: url,
            method: method || 'GET',
            dataType: 'json',
            data: payload
        }).then(function (response) {
            return resourceEnvelopeData(response);
        });
    }

    function fetchNodeRegistration(options) {
        var settings = $.extend({ refresh: false }, options || {});
        var nps = window.nps;
        if (!settings.refresh && nps._nodeRegistrationPromise) {
            return nps._nodeRegistrationPromise;
        }
        nps._nodeRegistrationPromise = fetchNodeData(nps.nodeRoute('registration'), 'GET').then(function (data) {
            nps._nodeRegistrationData = data || {};
            return nps._nodeRegistrationData;
        }, function (error) {
            if (settings.refresh) {
                nps._nodeRegistrationPromise = null;
            }
            throw error;
        });
        return nps._nodeRegistrationPromise;
    }

    function fetchNodeDisplay(options) {
        return fetchNodeRegistration(options).then(function (data) {
            return data && data.display ? data.display : {};
        });
    }

    function fetchNodeItem(url, transform) {
        return fetchNodeData(url, 'GET').then(function (data) {
            var item = data && data.item && typeof data.item === 'object' ? data.item : null;
            if (typeof transform === 'function') {
                return transform(item || {});
            }
            return item || {};
        });
    }

    function applyMappedFormGroups(mapping, activeKey, options) {
        mapping = mapping || {};
        options = options || {};
        var seen = {};
        Object.keys(mapping).forEach(function (key) {
            (mapping[key] || []).forEach(function (item) {
                if (!item || seen[item]) {
                    return;
                }
                seen[item] = true;
                setGroupVisibility('#' + item, false);
            });
        });
        (mapping[activeKey] || []).forEach(function (item) {
            if (!item) {
                return;
            }
            setGroupVisibility('#' + item, true);
        });
        if (options.caseContainer) {
            $(options.caseContainer).children().css('display', 'none');
            if (options.casePrefix) {
                setGroupVisibility(options.casePrefix + activeKey, true, 'inline');
            }
        }
        if (typeof options.after === 'function') {
            options.after(activeKey);
        }
    }

    function syncHostTLSFields(options) {
        options = options || {};
        var scheme = $(options.schemeSelector || '#scheme_select').val();
        var passthrough = $(options.passthroughSelector || '#https_just_proxy_select').val() === '1';
        var allowTLS = scheme === 'all' || scheme === 'https';
        setGroupVisibility(options.autoHTTPSSelector || '#auto_https', allowTLS);
        setGroupVisibility(options.justProxySelector || '#https_just_proxy', allowTLS);
        var showCertificates = allowTLS && !passthrough;
        setGroupVisibility(options.tlsOffloadSelector || '#tls_offload', showCertificates);
        setGroupVisibility(options.autoSSLSelector || '#auto_ssl', showCertificates);
        setGroupVisibility(options.certFileSelector || '#cert_file', showCertificates);
        setGroupVisibility(options.keyFileSelector || '#key_file', showCertificates);
    }

    function confirmMessage(action, count) {
        var langobj = languages['content'] && languages['content']['confirm'] ? languages['content']['confirm'][action] : null;
        var current = currentLang();
        var text = '';
        if ($.type(langobj) === 'object') {
            text = langobj[current] || langobj[languages['default']] || '';
        }
        if (!text) {
            text = uiLangText('info-confirm-generic');
        }
        if (count > 1) {
            text += uiLangFormat('info-selected-count-suffix', {count: count});
        }
        return text;
    }

    function selectionSummaryText(count) {
        count = parseInt(count, 10);
        if (isNaN(count) || count < 0) {
            count = 0;
        }
        return uiLangFormat('info-selected-count', {count: count});
    }

    function missingSelectionText() {
        return uiLangText('info-selection-required');
    }

    function batchFailedText() {
        return uiLangText('info-batch-failed');
    }

    function actionLabel(action) {
        switch (String(action || '').toLowerCase()) {
            case 'start':
                return uiLangText('info-batch-action-start');
            case 'stop':
                return uiLangText('info-batch-action-stop');
            case 'delete':
                return uiLangText('info-batch-action-delete');
            case 'clear':
                return uiLangText('info-batch-action-clear');
            default:
                return uiLangText('info-batch-action-generic');
        }
    }

    function batchSummaryText(action, total, success, failed) {
        var label = actionLabel(action);
        total = parseInt(total, 10);
        success = parseInt(success, 10);
        failed = parseInt(failed, 10);
        if (isNaN(total) || total < 0) total = 0;
        if (isNaN(success) || success < 0) success = 0;
        if (isNaN(failed) || failed < 0) failed = 0;
        if (failed > 0) {
            return uiLangFormat('info-batch-summary-failed', {
                label: label,
                total: total,
                success: success,
                failed: failed
            });
        }
        return uiLangFormat('info-batch-summary-success', {
            label: label,
            total: total,
            success: success,
            failed: failed
        });
    }

    function setDisabled(targets, disabled) {
        if (!targets) {
            return;
        }
        var $targets = $(targets);
        if (!$targets.length) {
            return;
        }
        $targets.prop('disabled', !!disabled).toggleClass('disabled', !!disabled);
    }

    function batchResponseItems(response) {
        if (!response) {
            return [];
        }
        if ($.isArray(response.items)) {
            return response.items;
        }
        if (response.data && $.isArray(response.data.items)) {
            return response.data.items;
        }
        return [];
    }

    function batchItemBody(item) {
        if (!item) {
            return null;
        }
        var body = item.body;
        if (typeof body === 'string') {
            try {
                return JSON.parse(body);
            } catch (e) {
                return body;
            }
        }
        return body;
    }

    function batchItemSucceeded(item) {
        if (!item) {
            return false;
        }
        if (item.error) {
            return false;
        }
        if (item.status && (item.status < 200 || item.status >= 300)) {
            return false;
        }
        var body = batchItemBody(item);
        if (body && typeof body === 'object' && Object.prototype.hasOwnProperty.call(body, 'status')) {
            return !!body.status;
        }
        return true;
    }

    function resourceListQueryParams(params, extra) {
        return $.extend({
            offset: params && params.offset ? params.offset : 0,
            limit: params && params.limit ? params.limit : 0,
            search: params && params.search ? params.search : '',
            sort: params && params.sort ? params.sort : '',
            order: params && params.order ? params.order : ''
        }, extra || {});
    }

    function resourceEnvelopeData(response) {
        if (response && response.data && response.meta && typeof response.data === 'object') {
            return response.data;
        }
        if (response && response.code === 1 && response.data && typeof response.data === 'object') {
            return response.data;
        }
        return {
            total: 0,
            items: []
        };
    }

    function resourceListResponseHandler(response, transform) {
        var data = resourceEnvelopeData(response);
        var items = $.isArray(data.items) ? data.items : [];
        if (typeof transform === 'function') {
            items = items.map(function (item) {
                return transform(item || {});
            });
        }
        return {
            total: data.total || 0,
            rows: items
        };
    }

    function megabytesFromBytes(value) {
        var bytes = parseInt(value, 10);
        if (isNaN(bytes) || bytes <= 0) {
            return 0;
        }
        return Math.round(bytes / (1024 * 1024));
    }

    function kilobytesFromBytesPerSecond(value) {
        var bytesPerSecond = parseInt(value, 10);
        if (isNaN(bytesPerSecond) || bytesPerSecond <= 0) {
            return 0;
        }
        return Math.ceil(bytesPerSecond / 1024);
    }

    function legacyTimeLimitValue(value) {
        var seconds = parseInt(value, 10);
        if (isNaN(seconds) || seconds <= 0) {
            return '0001-01-01T00:00:00Z';
        }
        return new Date(seconds * 1000).toISOString();
    }

    function adaptNodeUserResource(item) {
        item = item || {};
        return {
            Id: item.id || 0,
            Username: item.username || '',
            Kind: item.kind || '',
            ExternalPlatformID: item.external_platform_id || '',
            Hidden: !!item.hidden,
            Status: item.status || 0,
            ExpireAt: item.expire_at || 0,
            ExpireAtText: item.expire_at_text || '',
            FlowLimit: item.flow_limit_total_bytes || 0,
            TotalFlow: {
                InletFlow: item.total_in_bytes || 0,
                ExportFlow: item.total_out_bytes || 0
            },
            MaxClients: item.max_clients || 0,
            MaxTunnels: item.max_tunnels || 0,
            MaxHosts: item.max_hosts || 0,
            MaxConnections: item.max_connections || 0,
            RateLimit: kilobytesFromBytesPerSecond(item.rate_limit_total_bps),
            Rate: {
                NowRate: item.now_rate_total_bps || 0
            },
            EntryAclMode: item.entry_acl_mode || 0,
            EntryAclRules: item.entry_acl_rules || '',
            DestAclMode: item.dest_acl_mode || 0,
            DestAclRules: item.dest_acl_rules || '',
            Revision: item.revision || 0,
            UpdatedAt: item.updated_at || 0,
            ClientCount: item.client_count || 0,
            TunnelCount: item.tunnel_count || 0,
            HostCount: item.host_count || 0
        };
    }

    function adaptNodeClientResource(item) {
        item = item || {};
        var config = item.config || {};
        var inletFlow = item.total_in_bytes || 0;
        var exportFlow = item.total_out_bytes || 0;
        return {
            Id: item.id || 0,
            UserId: item.owner_user_id || 0,
            OwnerUserID: item.owner_user_id || 0,
            ManagerUserIDs: $.isArray(item.manager_user_ids) ? item.manager_user_ids.slice() : [],
            VerifyKey: item.verify_key || '',
            Mode: item.mode || '',
            Addr: item.addr || '',
            LocalAddr: item.local_addr || '',
            Remark: item.remark || '',
            Status: !!item.status,
            IsConnect: !!item.is_connect,
            NoStore: !!item.no_store,
            ExpireAt: item.expire_at || 0,
            FlowLimit: item.flow_limit_total_bytes || 0,
            RateLimit: kilobytesFromBytesPerSecond(item.rate_limit_total_bps),
            MaxConn: item.max_connections || 0,
            NowConn: item.now_conn || 0,
            ConnectionCount: item.connection_count || 0,
            MaxTunnelNum: item.max_tunnel_num || 0,
            ConfigConnAllow: !!item.config_conn_allow,
            Version: item.version || '',
            SourceType: item.source_type || '',
            SourcePlatformID: item.source_platform_id || '',
            SourceActorID: item.source_actor_id || '',
            Revision: item.revision || 0,
            UpdatedAt: item.updated_at || 0,
            Flow: {
                InletFlow: inletFlow,
                ExportFlow: exportFlow
            },
            InletFlow: inletFlow,
            ExportFlow: exportFlow,
            Rate: {
                NowRate: item.total_now_rate_total_bps || 0
            },
            BridgeRate: {
                NowRate: item.bridge_now_rate_total_bps || 0
            },
            ServiceRate: {
                NowRate: item.service_now_rate_total_bps || 0
            },
            EntryAclMode: item.entry_acl_mode || 0,
            EntryAclRules: item.entry_acl_rules || '',
            CreateTime: item.create_time || '',
            LastOnlineTime: item.last_online_time || '',
            Cnf: {
                U: config.user || '',
                P: config.password || '',
                Compress: !!config.compress,
                Crypt: !!config.crypt
            }
        };
    }

    function adaptNodeTunnelResource(item) {
        item = item || {};
        var inletFlow = item.service_in_bytes || 0;
        var exportFlow = item.service_out_bytes || 0;
        return {
            Id: item.id || 0,
            Revision: item.revision || 0,
            UpdatedAt: item.updated_at || 0,
            ClientId: item.client_id || 0,
            Client: adaptNodeClientResource(item.client || {}),
            Port: item.port || 0,
            ServerIp: item.server_ip || '',
            Mode: item.mode || '',
            Status: !!item.status,
            RunStatus: !!item.run_status,
            Remark: item.remark || '',
            TargetType: item.target_type || '',
            Password: item.password || '',
            LocalPath: item.local_path || '',
            StripPre: item.strip_pre || '',
            MaxConn: item.max_connections || 0,
            NowConn: item.now_conn || 0,
            IsHttp: !!item.is_http,
            HttpProxy: !!item.enable_http,
            Socks5Proxy: !!item.enable_socks5,
            ReadOnly: !!item.read_only,
            EntryAclMode: item.entry_acl_mode || 0,
            EntryAclRules: item.entry_acl_rules || '',
            DestAclMode: item.dest_acl_mode || 0,
            DestAclRules: item.dest_acl_rules || '',
            Target: {
                TargetStr: item.target || '',
                ProxyProtocol: item.proxy_protocol || 0,
                LocalProxy: !!item.local_proxy
            },
            InletFlow: inletFlow,
            ExportFlow: exportFlow,
            TotalFlow: inletFlow + exportFlow,
            Rate: {
                NowRate: item.now_rate_total_bps || 0
            },
            UserAuth: {
                Content: item.auth || ''
            },
            Flow: {
                InletFlow: inletFlow,
                ExportFlow: exportFlow,
                FlowLimit: megabytesFromBytes(item.flow_limit_total_bytes || 0),
                TimeLimit: legacyTimeLimitValue(item.expire_at)
            },
            RateLimit: kilobytesFromBytesPerSecond(item.rate_limit_total_bps)
        };
    }

    function adaptNodeHostResource(item) {
        item = item || {};
        var inletFlow = item.service_in_bytes || 0;
        var exportFlow = item.service_out_bytes || 0;
        return {
            Id: item.id || 0,
            Revision: item.revision || 0,
            UpdatedAt: item.updated_at || 0,
            ClientId: item.client_id || 0,
            Client: adaptNodeClientResource(item.client || {}),
            Host: item.host || '',
            HeaderChange: item.header || '',
            RespHeaderChange: item.resp_header || '',
            HostChange: item.host_change || '',
            Location: item.location || '',
            PathRewrite: item.path_rewrite || '',
            Remark: item.remark || '',
            Scheme: item.scheme || '',
            RedirectURL: item.redirect_url || '',
            HttpsJustProxy: !!item.https_just_proxy,
            TlsOffload: !!item.tls_offload,
            AutoSSL: !!item.auto_ssl,
            CertType: item.cert_type || '',
            CertHash: item.cert_hash || '',
            CertFile: item.cert_file || '',
            KeyFile: item.key_file || '',
            IsClose: !!item.is_close,
            AutoHttps: !!item.auto_https,
            AutoCORS: !!item.auto_cors,
            CompatMode: !!item.compat_mode,
            EntryAclMode: item.entry_acl_mode || 0,
            EntryAclRules: item.entry_acl_rules || '',
            TargetIsHttps: !!item.target_is_https,
            Target: {
                TargetStr: item.target || '',
                ProxyProtocol: item.proxy_protocol || 0,
                LocalProxy: !!item.local_proxy
            },
            InletFlow: inletFlow,
            ExportFlow: exportFlow,
            TotalFlow: inletFlow + exportFlow,
            NowConn: item.now_conn || 0,
            Rate: {
                NowRate: item.now_rate_total_bps || 0
            },
            UserAuth: {
                Content: item.auth || ''
            },
            Flow: {
                InletFlow: inletFlow,
                ExportFlow: exportFlow,
                FlowLimit: megabytesFromBytes(item.flow_limit_total_bytes || 0),
                TimeLimit: legacyTimeLimitValue(item.expire_at)
            },
            RateLimit: kilobytesFromBytesPerSecond(item.rate_limit_total_bps)
        };
    }

    function batchRequest(items, options) {
        options = options || {};
        return $.ajax({
            type: 'POST',
            url: options.url || window.nps.nodeRoute('batch'),
            dataType: 'json',
            contentType: 'application/json',
            data: JSON.stringify({items: normalizeBatchItems(items || [])}),
            headers: options.headers || {},
            processData: false
        });
    }

    function tableSelections(tableSelector) {
        var $table = $(tableSelector);
        if (!$table.length || typeof $table.bootstrapTable !== 'function') {
            return [];
        }
        try {
            return $table.bootstrapTable('getSelections') || [];
        } catch (e) {
            return [];
        }
    }

    function tableSelectedIDs(tableSelector, idField) {
        idField = idField || 'Id';
        return tableSelections(tableSelector).map(function (row) {
            return row ? row[idField] : null;
        }).filter(function (value) {
            return value !== null && value !== undefined && value !== '';
        });
    }

    function clearTableSelections(tableSelector) {
        var $table = $(tableSelector);
        if (!$table.length || typeof $table.bootstrapTable !== 'function') {
            return;
        }
        try {
            $table.bootstrapTable('uncheckAll');
        } catch (e) {}
    }

    function bindTableSelectionActions(options) {
        options = options || {};
        var tableSelector = options.table;
        var idField = options.idField || 'Id';
        var $table = $(tableSelector);
        if (!$table.length) {
            return function () { return []; };
        }
        var buttonTargets = options.buttons || [];
        var $count = $(options.count || []);
        var $summary = $(options.summary || []);
        var update = function () {
            var ids = tableSelectedIDs(tableSelector, idField);
            if ($count.length) {
                $count.text(ids.length);
            }
            if ($summary.length) {
                $summary.text(selectionSummaryText(ids.length));
            }
            setDisabled(buttonTargets, ids.length === 0);
            return ids;
        };
        $table.on('check.bs.table uncheck.bs.table check-all.bs.table uncheck-all.bs.table load-success.bs.table post-body.bs.table refresh.bs.table', update);
        window.setTimeout(update, 0);
        return update;
    }

    function runSelectionBatch(options) {
        options = options || {};
        var trigger = options.trigger || [];
        var ids = $.isArray(options.ids) ? options.ids.slice() : tableSelectedIDs(options.table, options.idField || 'Id');
        if (typeof options.filterIDs === 'function') {
            ids = options.filterIDs(ids);
        }
        if (!ids.length) {
            showMsg(missingSelectionText(), 'error', 2000);
            return $.Deferred().reject('no-selection').promise();
        }
        var prompt = options.confirmMessage;
        if (typeof prompt === 'function') {
            prompt = prompt(ids);
        }
        if (!prompt && options.confirmAction) {
            prompt = confirmMessage(options.confirmAction, ids.length);
        }
        if (prompt && !confirm(prompt)) {
            return $.Deferred().reject('cancelled').promise();
        }
        var items = typeof options.buildItems === 'function' ? (options.buildItems(ids) || []) : [];
        if (!items.length) {
            showMsg(missingSelectionText(), 'error', 2000);
            return $.Deferred().reject('empty-batch').promise();
        }
        setDisabled(trigger, true);
        return batchRequest(items, options.requestOptions).done(function (response) {
            var responseItems = batchResponseItems(response);
            var total = responseItems.length || items.length;
            var success = 0;
            responseItems.forEach(function (item) {
                if (batchItemSucceeded(item)) {
                    success += 1;
                }
            });
            if (!responseItems.length) {
                success = total;
            }
            var failed = Math.max(total - success, 0);
            var message = batchSummaryText(options.action || options.confirmAction, total, success, failed);
            var type = failed > 0 ? 'error' : 'success';
            var after = function () {
                if (success > 0 && typeof options.refresh === 'function') {
                    options.refresh({
                        ids: ids,
                        items: items,
                        responseItems: responseItems,
                        success: success,
                        failed: failed
                    });
                }
            };
            showMsg(message, type, failed > 0 ? 3500 : 1500, after);
        }).fail(function () {
            showMsg(batchFailedText(), 'error', 3500);
        }).always(function () {
            setDisabled(trigger, false);
        });
    }

    function normalizeSubmitPayload(postdata) {
        if ($.isArray(postdata)) {
            return $.map(postdata, function (item) {
                if (!item) {
                    return null;
                }
                var copy = $.extend({}, item);
                if (typeof copy.value === 'string') {
                    copy.value = copy.value.trim();
                }
                return copy;
            });
        }
        if ($.isPlainObject(postdata)) {
            var normalized = {};
            $.each(postdata, function (key, value) {
                normalized[key] = typeof value === 'string' ? value.trim() : value;
            });
            return normalized;
        }
        return postdata;
    }

    function formArrayToObject(postdata) {
        if (!$.isArray(postdata)) {
            return $.isPlainObject(postdata) ? $.extend({}, postdata) : {};
        }
        var result = {};
        $.each(postdata, function (_, item) {
            if (!item || !item.name) {
                return;
            }
            var value = typeof item.value === 'string' ? item.value.trim() : item.value;
            if (Object.prototype.hasOwnProperty.call(result, item.name)) {
                if (!$.isArray(result[item.name])) {
                    result[item.name] = [result[item.name]];
                }
                result[item.name].push(value);
                return;
            }
            result[item.name] = value;
        });
        return result;
    }

    function integerFieldValue(value) {
        if (value === null || value === undefined) {
            return '';
        }
        return $.trim(String(value));
    }

    function parseOptionalIntegerField(value) {
        if (typeof value === 'number' && isFinite(value)) {
            return value;
        }
        var text = integerFieldValue(value);
        if (!text) {
            return undefined;
        }
        var parsed = parseInt(text, 10);
        if (isNaN(parsed)) {
            return undefined;
        }
        return parsed;
    }

    function parseLimitIntegerField(value) {
        var parsed = parseOptionalIntegerField(value);
        return parsed === undefined ? 0 : parsed;
    }

    function parseBooleanField(value) {
        if (value === true || value === false) {
            return value;
        }
        var text = $.trim(String(value === null || value === undefined ? '' : value)).toLowerCase();
        return text === '1' || text === 'true' || text === 'yes' || text === 'on';
    }

    function parseIntegerListField(value) {
        if ($.isArray(value)) {
            return value.map(parseOptionalIntegerField).filter(function (item) {
                return item !== undefined && item > 0;
            });
        }
        var text = $.trim(String(value === null || value === undefined ? '' : value));
        if (!text) {
            return [];
        }
        var seen = {};
        return text.split(/[\s,]+/).map(parseOptionalIntegerField).filter(function (item) {
            if (item === undefined || item <= 0 || seen[item]) {
                return false;
            }
            seen[item] = true;
            return true;
        });
    }

    function parseRateLimitBpsField(value, legacyKilobytes) {
        var parsed = parseLimitIntegerField(value);
        if (parsed <= 0) {
            return 0;
        }
        return legacyKilobytes ? parsed * 1024 : parsed;
    }

    function parseFlowLimitBytesField(value, legacyMegabytes) {
        var parsed = parseLimitIntegerField(value);
        if (parsed <= 0) {
            return 0;
        }
        return legacyMegabytes ? parsed * 1024 * 1024 : parsed;
    }

    function firstDefinedValue(source, keys) {
        if (!source || !keys || !keys.length) {
            return undefined;
        }
        for (var i = 0; i < keys.length; i += 1) {
            var key = keys[i];
            if (Object.prototype.hasOwnProperty.call(source, key)) {
                return source[key];
            }
        }
        return undefined;
    }

    function assignIfDefined(target, key, value) {
        if (value !== undefined) {
            target[key] = value;
        }
    }

    function hasAnyPayloadKey(source, keys) {
        if (!source || !keys || !keys.length) {
            return false;
        }
        for (var i = 0; i < keys.length; i += 1) {
            if (Object.prototype.hasOwnProperty.call(source, keys[i])) {
                return true;
            }
        }
        return false;
    }

    function normalizeFormalNodePayload(url, postdata) {
        var payload = formArrayToObject(normalizeSubmitPayload(postdata));
        if (!$.isPlainObject(payload)) {
            return payload;
        }
        var parts = splitRequestURL(String(url || ''));
        var path = normalizeLegacyNodePath(parts.path);
        var result = null;

        if (/\/api\/settings\/global(?:\/actions\/update)?\/?$/i.test(path)) {
            result = {
                entry_acl_mode: parseLimitIntegerField(firstDefinedValue(payload, ['entry_acl_mode'])),
                entry_acl_rules: $.trim(String(firstDefinedValue(payload, ['entry_acl_rules']) || ''))
            };
            return result;
        }

        if (/\/api\/security\/bans\/actions\/delete\/?$/i.test(path)) {
            return {
                key: $.trim(String(firstDefinedValue(payload, ['key']) || ''))
            };
        }

        if (/\/api\/(?:users|clients)\/[^/?#]+\/actions\/status\/?$/i.test(path)) {
            result = $.extend({}, payload);
            if (Object.prototype.hasOwnProperty.call(result, 'status')) {
                result.status = parseBooleanField(result.status);
            }
            return result;
        }

        if (/\/api\/users(?:\/[^/?#]+\/actions\/update)?\/?$/i.test(path)) {
            result = {
                username: $.trim(String(firstDefinedValue(payload, ['username']) || '')),
                status: parseBooleanField(firstDefinedValue(payload, ['status'])),
                expire_at: $.trim(String(firstDefinedValue(payload, ['expire_at']) || '')),
                flow_limit_total_bytes: parseFlowLimitBytesField(firstDefinedValue(payload, ['flow_limit_total_bytes', 'flow_limit']), false),
                rate_limit_total_bps: parseRateLimitBpsField(firstDefinedValue(payload, ['rate_limit_total_bps', 'rate_limit']), !Object.prototype.hasOwnProperty.call(payload, 'rate_limit_total_bps')),
                max_clients: parseLimitIntegerField(firstDefinedValue(payload, ['max_clients'])),
                max_tunnels: parseLimitIntegerField(firstDefinedValue(payload, ['max_tunnels'])),
                max_hosts: parseLimitIntegerField(firstDefinedValue(payload, ['max_hosts'])),
                max_connections: parseLimitIntegerField(firstDefinedValue(payload, ['max_connections'])),
                reset_flow: parseBooleanField(firstDefinedValue(payload, ['reset_flow', 'flow_reset'])),
                entry_acl_mode: parseLimitIntegerField(firstDefinedValue(payload, ['entry_acl_mode'])),
                entry_acl_rules: $.trim(String(firstDefinedValue(payload, ['entry_acl_rules']) || '')),
                dest_acl_mode: parseLimitIntegerField(firstDefinedValue(payload, ['dest_acl_mode'])),
                dest_acl_rules: $.trim(String(firstDefinedValue(payload, ['dest_acl_rules']) || ''))
            };
            if (Object.prototype.hasOwnProperty.call(payload, 'password')) {
                result.password = String(payload.password === null || payload.password === undefined ? '' : payload.password);
            }
            if (Object.prototype.hasOwnProperty.call(payload, 'totp_secret')) {
                result.totp_secret = String(payload.totp_secret === null || payload.totp_secret === undefined ? '' : payload.totp_secret);
            }
            return result;
        }

        if (/\/api\/clients(?:\/[^/?#]+\/actions\/update)?\/?$/i.test(path)) {
            result = {
                verify_key: $.trim(String(firstDefinedValue(payload, ['verify_key', 'vkey']) || '')),
                remark: $.trim(String(firstDefinedValue(payload, ['remark']) || '')),
                username: $.trim(String(firstDefinedValue(payload, ['username', 'u']) || '')),
                compress: parseBooleanField(firstDefinedValue(payload, ['compress'])),
                crypt: parseBooleanField(firstDefinedValue(payload, ['crypt'])),
                config_conn_allow: parseBooleanField(firstDefinedValue(payload, ['config_conn_allow'])),
                rate_limit_total_bps: parseRateLimitBpsField(firstDefinedValue(payload, ['rate_limit_total_bps', 'rate_limit']), !Object.prototype.hasOwnProperty.call(payload, 'rate_limit_total_bps')),
                max_connections: parseLimitIntegerField(firstDefinedValue(payload, ['max_connections', 'max_conn'])),
                max_tunnel_num: parseLimitIntegerField(firstDefinedValue(payload, ['max_tunnel_num', 'max_tunnel'])),
                flow_limit_total_bytes: parseFlowLimitBytesField(firstDefinedValue(payload, ['flow_limit_total_bytes', 'flow_limit']), false),
                expire_at: $.trim(String(firstDefinedValue(payload, ['expire_at', 'time_limit']) || '')),
                entry_acl_mode: parseLimitIntegerField(firstDefinedValue(payload, ['entry_acl_mode'])),
                entry_acl_rules: $.trim(String(firstDefinedValue(payload, ['entry_acl_rules']) || '')),
                reset_flow: parseBooleanField(firstDefinedValue(payload, ['reset_flow', 'flow_reset']))
            };
            if (hasAnyPayloadKey(payload, ['password', 'p'])) {
                result.password = String(firstDefinedValue(payload, ['password', 'p']) || '');
            }
            var ownerUserID = parseOptionalIntegerField(firstDefinedValue(payload, ['owner_user_id', 'user_id']));
            assignIfDefined(result, 'owner_user_id', ownerUserID);
            if (Object.prototype.hasOwnProperty.call(payload, 'manager_user_ids')) {
                result.manager_user_ids = parseIntegerListField(payload.manager_user_ids);
            }
            return result;
        }

        if (/\/api\/tunnels(?:\/[^/?#]+\/actions\/update)?\/?$/i.test(path)) {
            result = {
                client_id: parseLimitIntegerField(firstDefinedValue(payload, ['client_id'])),
                port: parseLimitIntegerField(firstDefinedValue(payload, ['port'])),
                server_ip: $.trim(String(firstDefinedValue(payload, ['server_ip']) || '')),
                mode: $.trim(String(firstDefinedValue(payload, ['mode', 'type']) || '')),
                target_type: $.trim(String(firstDefinedValue(payload, ['target_type']) || '')),
                target: $.trim(String(firstDefinedValue(payload, ['target']) || '')),
                proxy_protocol: parseLimitIntegerField(firstDefinedValue(payload, ['proxy_protocol'])),
                local_proxy: parseBooleanField(firstDefinedValue(payload, ['local_proxy'])),
                auth: String(firstDefinedValue(payload, ['auth']) || ''),
                remark: $.trim(String(firstDefinedValue(payload, ['remark']) || '')),
                password: String(firstDefinedValue(payload, ['password']) || ''),
                local_path: $.trim(String(firstDefinedValue(payload, ['local_path']) || '')),
                strip_pre: $.trim(String(firstDefinedValue(payload, ['strip_pre']) || '')),
                enable_http: parseBooleanField(firstDefinedValue(payload, ['enable_http'])),
                enable_socks5: parseBooleanField(firstDefinedValue(payload, ['enable_socks5'])),
                entry_acl_mode: parseLimitIntegerField(firstDefinedValue(payload, ['entry_acl_mode'])),
                entry_acl_rules: $.trim(String(firstDefinedValue(payload, ['entry_acl_rules']) || '')),
                dest_acl_mode: parseLimitIntegerField(firstDefinedValue(payload, ['dest_acl_mode'])),
                dest_acl_rules: $.trim(String(firstDefinedValue(payload, ['dest_acl_rules']) || '')),
                flow_limit_total_bytes: parseFlowLimitBytesField(firstDefinedValue(payload, ['flow_limit_total_bytes', 'flow_limit']), !Object.prototype.hasOwnProperty.call(payload, 'flow_limit_total_bytes')),
                expire_at: $.trim(String(firstDefinedValue(payload, ['expire_at', 'time_limit']) || '')),
                reset_flow: parseBooleanField(firstDefinedValue(payload, ['reset_flow', 'flow_reset']))
            };
            if (hasAnyPayloadKey(payload, ['rate_limit_total_bps', 'rate_limit'])) {
                result.rate_limit_total_bps = parseRateLimitBpsField(firstDefinedValue(payload, ['rate_limit_total_bps', 'rate_limit']), !Object.prototype.hasOwnProperty.call(payload, 'rate_limit_total_bps'));
            }
            if (hasAnyPayloadKey(payload, ['max_connections', 'max_conn'])) {
                result.max_connections = parseLimitIntegerField(firstDefinedValue(payload, ['max_connections', 'max_conn']));
            }
            return result;
        }

        if (/\/api\/hosts(?:\/[^/?#]+\/actions\/update)?\/?$/i.test(path)) {
            result = {
                client_id: parseLimitIntegerField(firstDefinedValue(payload, ['client_id'])),
                host: $.trim(String(firstDefinedValue(payload, ['host']) || '')),
                target: $.trim(String(firstDefinedValue(payload, ['target']) || '')),
                proxy_protocol: parseLimitIntegerField(firstDefinedValue(payload, ['proxy_protocol'])),
                local_proxy: parseBooleanField(firstDefinedValue(payload, ['local_proxy'])),
                auth: String(firstDefinedValue(payload, ['auth']) || ''),
                header: String(firstDefinedValue(payload, ['header']) || ''),
                resp_header: String(firstDefinedValue(payload, ['resp_header']) || ''),
                host_change: $.trim(String(firstDefinedValue(payload, ['host_change', 'hostchange']) || '')),
                remark: $.trim(String(firstDefinedValue(payload, ['remark']) || '')),
                location: $.trim(String(firstDefinedValue(payload, ['location']) || '')),
                path_rewrite: $.trim(String(firstDefinedValue(payload, ['path_rewrite']) || '')),
                redirect_url: $.trim(String(firstDefinedValue(payload, ['redirect_url']) || '')),
                flow_limit_total_bytes: parseFlowLimitBytesField(firstDefinedValue(payload, ['flow_limit_total_bytes', 'flow_limit']), !Object.prototype.hasOwnProperty.call(payload, 'flow_limit_total_bytes')),
                expire_at: $.trim(String(firstDefinedValue(payload, ['expire_at', 'time_limit']) || '')),
                reset_flow: parseBooleanField(firstDefinedValue(payload, ['reset_flow', 'flow_reset'])),
                entry_acl_mode: parseLimitIntegerField(firstDefinedValue(payload, ['entry_acl_mode'])),
                entry_acl_rules: $.trim(String(firstDefinedValue(payload, ['entry_acl_rules']) || '')),
                scheme: $.trim(String(firstDefinedValue(payload, ['scheme']) || '')),
                https_just_proxy: parseBooleanField(firstDefinedValue(payload, ['https_just_proxy'])),
                tls_offload: parseBooleanField(firstDefinedValue(payload, ['tls_offload'])),
                auto_ssl: parseBooleanField(firstDefinedValue(payload, ['auto_ssl'])),
                key_file: String(firstDefinedValue(payload, ['key_file']) || ''),
                cert_file: String(firstDefinedValue(payload, ['cert_file']) || ''),
                auto_https: parseBooleanField(firstDefinedValue(payload, ['auto_https'])),
                auto_cors: parseBooleanField(firstDefinedValue(payload, ['auto_cors'])),
                compat_mode: parseBooleanField(firstDefinedValue(payload, ['compat_mode'])),
                target_is_https: parseBooleanField(firstDefinedValue(payload, ['target_is_https'])),
                sync_cert_to_matching_hosts: parseBooleanField(firstDefinedValue(payload, ['sync_cert_to_matching_hosts']))
            };
            if (hasAnyPayloadKey(payload, ['rate_limit_total_bps', 'rate_limit'])) {
                result.rate_limit_total_bps = parseRateLimitBpsField(firstDefinedValue(payload, ['rate_limit_total_bps', 'rate_limit']), !Object.prototype.hasOwnProperty.call(payload, 'rate_limit_total_bps'));
            }
            if (hasAnyPayloadKey(payload, ['max_connections', 'max_conn'])) {
                result.max_connections = parseLimitIntegerField(firstDefinedValue(payload, ['max_connections', 'max_conn']));
            }
            return result;
        }

        return payload;
    }

    function resolveSubmitRequest(url, postdata) {
        if ($.isPlainObject(url) && url.url) {
            return {
                method: String(url.method || url.type || 'POST').toUpperCase(),
                url: url.url,
                data: url.data !== undefined ? url.data : postdata
            };
        }
        return {
            method: 'POST',
            url: url,
            data: postdata
        };
    }

    function splitRequestURL(url) {
        var value = String(url || '');
        var match = value.match(/^([^?#]*)([?#].*)?$/);
        return {
            path: match ? (match[1] || '') : value,
            suffix: match ? (match[2] || '') : ''
        };
    }

    function trimTrailingSlash(path) {
        return String(path || '').replace(/\/+$/, '');
    }

    function normalizeLegacyNodePath(path) {
        path = String(path || '');
        path = path.replace(/\/api\/banlist\/unban-all\/?$/i, '/api/security/bans/actions/delete_all');
        path = path.replace(/\/api\/banlist\/clean\/?$/i, '/api/security/bans/actions/clean');
        path = path.replace(/\/api\/banlist\/([^/?#]+)\/unban\/?$/i, '/api/security/bans/actions/delete');
        return path;
    }

    function isFormalNodeMutationURL(url) {
        return /\/api\/(?:users|clients|tunnels|hosts|settings\/global|security\/bans|system\/import|banlist)(?:\/|$)/i.test(String(url || ''));
    }

    function normalizeFormalNodeRequest(request) {
        var normalized = $.extend({}, request || {});
        normalized.method = String(normalized.method || normalized.type || 'POST').toUpperCase();
        normalized.url = String(normalized.url || '');
        if (!isFormalNodeMutationURL(normalized.url)) {
            return normalized;
        }
        var parts = splitRequestURL(normalized.url);
        var path = normalizeLegacyNodePath(parts.path);
        if (normalized.method === 'PATCH') {
            if (/\/api\/settings\/global\/?$/i.test(path)) {
                path = trimTrailingSlash(path) + '/actions/update';
                normalized.method = 'POST';
            } else if (/\/api\/(?:users|clients|tunnels|hosts)\/[^/?#]+\/?$/i.test(path)) {
                path = trimTrailingSlash(path) + '/actions/update';
                normalized.method = 'POST';
            }
        } else if (normalized.method === 'DELETE' && /\/api\/(?:users|clients|tunnels|hosts)\/[^/?#]+\/?$/i.test(path)) {
            path = trimTrailingSlash(path) + '/actions/delete';
            normalized.method = 'POST';
        }
        normalized.url = path + parts.suffix;
        return normalized;
    }

    function normalizeBatchItems(items) {
        return $.map(items || [], function (item) {
            if (!item) {
                return null;
            }
            var normalized = $.extend({}, item);
            var request = normalizeFormalNodeRequest({
                method: normalized.method || normalized.type || 'POST',
                url: normalized.path || normalized.url || ''
            });
            normalized.method = request.method;
            normalized.path = request.url;
            if (Object.prototype.hasOwnProperty.call(normalized, 'url')) {
                normalized.url = request.url;
            }
            return normalized;
        });
    }

    function submitAjaxOptions(request) {
        request = normalizeFormalNodeRequest(request);
        var options = {
            type: request.method || 'POST',
            method: request.method || 'POST',
            url: request.url
        };
        if (isFormalNodeMutationURL(options.url)) {
            options.contentType = 'application/json';
            options.processData = false;
            options.dataType = 'json';
            options.data = JSON.stringify(normalizeFormalNodePayload(options.url, request.data));
            return options;
        }
        options.data = normalizeSubmitPayload(request.data);
        return options;
    }

    function mutationSucceeded(res) {
        if (!res) {
            return false;
        }
        if (Object.prototype.hasOwnProperty.call(res, 'error')) {
            return false;
        }
        if (Object.prototype.hasOwnProperty.call(res, 'meta') && Object.prototype.hasOwnProperty.call(res, 'data')) {
            return true;
        }
        if (Object.prototype.hasOwnProperty.call(res, 'code')) {
            return Number(res.code) === 1;
        }
        if (Object.prototype.hasOwnProperty.call(res, 'status')) {
            return !!res.status;
        }
        return false;
    }

    function mutationMessage(res, fallback) {
        if (res && res.data && typeof res.data === 'object' && res.data.message) {
            return langreply(res.data.message);
        }
        if (res && res.msg) {
            return langreply(res.msg);
        }
        if (res && res.error && typeof res.error === 'object' && res.error.message) {
            return langreply(res.error.message);
        }
        return fallback;
    }

    function mutationErrorMessage(xhr, fallback) {
        var responseJSON = xhr && xhr.responseJSON;
        if (responseJSON && responseJSON.error && typeof responseJSON.error === 'object' && responseJSON.error.message) {
            return langreply(responseJSON.error.message);
        }
        if (responseJSON && responseJSON.msg) {
            return langreply(responseJSON.msg);
        }
        if (xhr && xhr.responseText) {
            try {
                var decoded = JSON.parse(xhr.responseText);
                if (decoded && decoded.error && typeof decoded.error === 'object' && decoded.error.message) {
                    return langreply(decoded.error.message);
                }
                if (decoded && decoded.msg) {
                    return langreply(decoded.msg);
                }
            } catch (e) {}
        }
        return fallback;
    }

    function defaultMutationMessage(kind) {
        if (kind === 'error') {
            return window.nps.langText('info-request-failed', '');
        }
        return window.nps.langText('info-request-succeeded', '');
    }

    function submitMutationRequest(request, onSuccess) {
        var requestOptions = submitAjaxOptions(request);
        var successMessage = defaultMutationMessage('success');
        var errorMessage = defaultMutationMessage('error');
        $.ajax({
            type: requestOptions.type,
            method: requestOptions.method,
            url: requestOptions.url,
            contentType: requestOptions.contentType,
            processData: requestOptions.processData,
            dataType: requestOptions.dataType,
            data: requestOptions.data,
            success: function (res) {
                if (mutationSucceeded(res)) {
                    showMsg(mutationMessage(res, successMessage), 'success', 1000, function () {
                        if (typeof onSuccess === 'function') {
                            onSuccess(res);
                        }
                    });
                    return;
                }
                showMsg(mutationMessage(res, errorMessage), 'error', 5000);
            },
            error: function (xhr) {
                showMsg(mutationErrorMessage(xhr, errorMessage), 'error', 5000);
            }
        });
    }

    function confirmFormAction(action, count) {
        var normalized = String(action || '').toLowerCase();
        if (normalized !== 'turn' && normalized !== 'clear' && normalized !== 'delete') {
            return true;
        }
        var langobj = languages['content'] && languages['content']['confirm'] ? languages['content']['confirm'][normalized] : null;
        var current = currentLang();
        var text = '';
        if ($.type(langobj) === 'object') {
            text = langobj[current] || langobj[languages['default']] || '';
        }
        if (!text) {
            text = uiLangText('info-confirm-generic');
        }
        if (typeof count === 'number' && count > 1) {
            text += uiLangFormat('info-selected-count-suffix', {count: count});
        }
        return confirm(text);
    }

    function submitFormAction(action, url, postdata) {
        var request = resolveSubmitRequest(url, postdata);
        var normalizedAction = String(action || '').toLowerCase();
        var reloadAfterSuccess = normalizedAction === 'start' || normalizedAction === 'stop';
        if (!confirmFormAction(normalizedAction)) {
            return;
        }
        switch (normalizedAction) {
            case 'add':
            case 'edit':
            case 'start':
            case 'stop':
            case 'turn':
            case 'clear':
            case 'delete':
                submitMutationRequest(request, function () {
                    if (reloadAfterSuccess) {
                        document.location.reload();
                        return;
                    }
                    window.location.href = document.referrer;
                });
                return;
            case 'global':
                submitMutationRequest(request, function () {
                    document.location.reload();
                });
                return;
            default:
                submitMutationRequest(request);
        }
    }

    var nps = window.nps;
    nps.initRemoteSelectPicker = initRemoteSelectPicker;
    nps.setGroupVisibility = setGroupVisibility;
    nps.pageParam = pageParam;
    nps.pageIntParam = pageIntParam;
    nps.setSelectValue = setSelectValue;
    nps.setFormValue = setFormValue;
    nps.setTextContent = setTextContent;
    nps.fetchNodeItem = fetchNodeItem;
    nps.fetchNodeData = fetchNodeData;
    nps.fetchNodeRegistration = fetchNodeRegistration;
    nps.fetchNodeDisplay = fetchNodeDisplay;
    nps.applyMappedFormGroups = applyMappedFormGroups;
    nps.syncHostTLSFields = syncHostTLSFields;
    nps.applyReadOnlyForm = applyReadOnlyForm;
    nps.isReadOnlyForm = isReadOnlyForm;
    nps.selectionSummaryText = selectionSummaryText;
    nps.resourceListQueryParams = resourceListQueryParams;
    nps.resourceListResponseHandler = resourceListResponseHandler;
    nps.adaptNodeUserResource = adaptNodeUserResource;
    nps.adaptNodeClientResource = adaptNodeClientResource;
    nps.adaptNodeTunnelResource = adaptNodeTunnelResource;
    nps.adaptNodeHostResource = adaptNodeHostResource;
    nps.batchRequest = batchRequest;
    nps.tableSelections = tableSelections;
    nps.tableSelectedIDs = tableSelectedIDs;
    nps.clearTableSelections = clearTableSelections;
    nps.bindTableSelectionActions = bindTableSelectionActions;
    nps.runSelectionBatch = runSelectionBatch;
    nps.submitFormAction = submitFormAction;
})(jQuery);
