(function ($) {
    function xml2json(Xml) {
        var tempvalue, tempJson = {};
        $(Xml).each(function () {
            var tagName = ($(this).attr('id') || this.tagName);
            tempvalue = (this.childElementCount == 0) ? this.textContent : xml2json($(this).children());
            switch ($.type(tempJson[tagName])) {
                case 'undefined':
                    tempJson[tagName] = tempvalue;
                    break;
                case 'object':
                    tempJson[tagName] = Array(tempJson[tagName]);
                case 'array':
                    tempJson[tagName].push(tempvalue);
            }
        });
        return tempJson;
    }

    function setCookie(c_name, value, expiredays) {
        var exdate = new Date();
        exdate.setDate(exdate.getDate() + expiredays);
        document.cookie = c_name + '=' + escape(value) + ((expiredays == null) ? '' : ';expires=' + exdate.toGMTString()) + '; path=' + window.nps.web_base_url + '/;';
    }

    function getCookie(c_name) {
        if (document.cookie.length > 0) {
            var c_start = document.cookie.indexOf(c_name + '=');
            if (c_start != -1) {
                c_start = c_start + c_name.length + 1;
                var c_end = document.cookie.indexOf(';', c_start);
                if (c_end == -1) {
                    c_end = document.cookie.length;
                }
                return unescape(document.cookie.substring(c_start, c_end));
            }
        }
        return null;
    }

    function setchartlang(langobj, chartobj) {
        if ($.type(langobj) == 'string') return langobj;
        if ($.type(langobj) == 'chartobj') return false;
        var flag = true;
        for (var key in langobj) {
            var item = key;
            var children = (chartobj.hasOwnProperty(item)) ? setchartlang(langobj[item], chartobj[item]) : setchartlang(langobj[item], undefined);
            switch ($.type(children)) {
                case 'string':
                    if ($.type(chartobj[item]) != 'string') continue;
                case 'object':
                    chartobj[item] = (children['value'] || children);
                default:
                    flag = false;
            }
        }
        if (flag) {
            return {'value': (langobj[languages['current']] || langobj[languages['default']] || 'N/A')};
        }
    }

    $.fn.cloudLang = function () {
        $.ajax({
            type: 'GET',
            url: window.nps.web_base_url + '/static/page/languages.xml?v=' + window.nps.version,
            dataType: 'xml',
            success: function (xml) {
                languages['content'] = xml2json($(xml).children())['content'];
                languages['menu'] = languages['content']['languages'];
                languages['default'] = languages['content']['default'];
                var navLang = (getCookie('lang') || navigator.language || navigator.browserLanguage || '');
                languages['navigator'] = navLang.startsWith('zh') ? 'zh-CN' : navLang;
                for (var key in languages['menu']) {
                    $('#languagemenu').next().append('<li lang="' + key + '"><a><img src="' + window.nps.web_base_url + '/static/img/flag/' + key + '.png"> ' + languages['menu'][key] + '</a></li>');
                    if (key == languages['navigator']) {
                        languages['current'] = key;
                    }
                }
                $('#languagemenu').attr('lang', (languages['current'] || languages['default']));
                $('body').setLang('');

                if ($.fn.selectpicker != null) {
                    $('.selectpicker').selectpicker('refresh');
                }
            }
        });
    };

    $.fn.setLang = function (dom) {
        languages['current'] = $('#languagemenu').attr('lang');
        if (dom == '') {
            $('#languagemenu span').text(' ' + languages['menu'][languages['current']]);
            if (languages['current'] != getCookie('lang')) {
                setCookie('lang', languages['current']);
            }
            if ($("#table").length > 0) {
                $('#table').bootstrapTable('refreshOptions', {'locale': languages['current']});
            }
            if (window.nps && typeof window.nps.syncThemeToggle === 'function') {
                window.nps.syncThemeToggle();
            }
        }
        $.each($(dom + ' [langtag]'), function (i, item) {
            var index = $(item).attr('langtag');
            var string = languages['content'][index.toLowerCase()];
            switch ($.type(string)) {
                case 'string':
                    break;
                case 'array':
                    string = string[Math.floor((Math.random() * string.length))];
                case 'object':
                    string = (string[languages['current']] || string[languages['default']] || null);
                    break;
                default:
                    string = '';
                    $(item).css('background-color', '#ffeeba');
            }
            var attrList = String($(item).attr('langattr') || '').split(',').map(function (attr) {
                return $.trim(attr);
            }).filter(Boolean);
            if (attrList.length > 0) {
                attrList.forEach(function (attr) {
                    $(item).attr(attr, string);
                });
            } else if ($.type($(item).attr('placeholder')) == 'undefined') {
                $(item).text(string);
            } else {
                $(item).attr('placeholder', string);
            }
        });

        if (!$.isEmptyObject(chartdatas)) {
            setchartlang(languages['content']['charts'], chartdatas);
            for (var key in chartdatas) {
                if ($('#' + key).length == 0) continue;
                if ($.type(chartdatas[key]) == 'object') {
                    charts[key] = echarts.init(document.getElementById(key));
                }
                charts[key].setOption(chartdatas[key], true);
            }
        }

        if (window.hasOwnProperty('internationalized')) {
            internationalized(languages['current']);
        }
    };
})(jQuery);

var languages = {};
var charts = {};
var chartdatas = {};

function langreply(langstr) {
    var langobj = languages['content']['reply'][langstr.replace(/[\s,.?]*/g, "").toLowerCase()];
    if ($.type(langobj) == 'undefined') return langstr;
    langobj = (langobj[languages['current']] || langobj[languages['default']] || langstr);
    return langobj;
}

(function ($) {
    function preferredLang() {
        var match = document.cookie && document.cookie.match(/(?:^|;\s*)lang=([^;]+)/);
        var cookieLang = match ? decodeURIComponent(match[1]) : '';
        var navLang = cookieLang || navigator.language || navigator.browserLanguage || '';
        return /^zh/i.test(String(navLang)) ? 'zh-CN' : (navLang || 'en-US');
    }

    function resolveLangNode(path) {
        var content = languages['content'];
        if (!content || !path) {
            return null;
        }
        var node = content;
        String(path).toLowerCase().split('.').forEach(function (segment) {
            if (node != null) {
                node = node[segment];
            }
        });
        return node == null ? null : node;
    }

    function resolveLangValue(node, fallback) {
        switch ($.type(node)) {
            case 'string':
                return node;
            case 'array':
                return node.length ? resolveLangValue(node[0], fallback) : (fallback || '');
            case 'object':
                return node[currentLang()] || node[languages['default']] || fallback || '';
            default:
                return fallback || '';
        }
    }

    function langText(path, fallback) {
        var hasFallback = arguments.length > 1;
        return resolveLangValue(resolveLangNode(path), hasFallback ? fallback : (path || ''));
    }

    function langFormat(path, replacements, fallback) {
        var hasFallback = arguments.length > 2;
        var template = String(langText(path, hasFallback ? fallback : undefined));
        return template.replace(/\{([a-z0-9_]+)\}/gi, function (match, key) {
            if (replacements && Object.prototype.hasOwnProperty.call(replacements, key)) {
                return String(replacements[key]);
            }
            return match;
        });
    }

    function currentLang() {
        return (languages['current'] || languages['default'] || preferredLang());
    }

    function isZhLanguage() {
        return /^zh/i.test(String(currentLang()));
    }

    var nps = window.nps;
    nps.currentLang = currentLang;
    nps.isZhLanguage = isZhLanguage;
    nps.langText = langText;
    nps.langFormat = langFormat;

    $(document).ready(function () {
        window.nps.syncThemeToggle();
        $('body').cloudLang();
        window.nps.enhancePageLinks(document);
        $('body').on('click', 'li[lang]', function () {
            $('#languagemenu').attr('lang', $(this).attr('lang'));
            $('body').setLang('');
        });
    });
})(jQuery);
