(function () {
    function langTextSafe(path) {
        return window.nps.langText(path, '');
    }

    function feedbackLangText(path, fallback) {
        if (window.nps && typeof window.nps.langText === 'function') {
            return window.nps.langText(path, fallback);
        }
        return fallback || '';
    }

    function unitText(path) {
        return langTextSafe(path);
    }

    function formatCompactDurationParts(days, hours, minutes, seconds, maxParts) {
        var suffixes = {
            day: langTextSafe('word-day-short'),
            hour: langTextSafe('word-hour-short'),
            minute: langTextSafe('word-minute-short'),
            second: langTextSafe('word-second-short')
        };
        var parts = [];
        if (days > 0) {
            parts.push(days + suffixes.day);
        }
        if (hours > 0) {
            parts.push(hours + suffixes.hour);
        }
        if (minutes > 0) {
            parts.push(minutes + suffixes.minute);
        }
        if (seconds > 0 || parts.length === 0) {
            parts.push(seconds + suffixes.second);
        }
        if (typeof maxParts === 'number' && maxParts > 0 && parts.length > maxParts) {
            return parts.slice(0, maxParts);
        }
        return parts;
    }

    function formatCompactDuration(totalSeconds, maxParts) {
        if (totalSeconds === Infinity) {
            return '∞';
        }
        var secs = Number(totalSeconds);
        if (!isFinite(secs) || secs < 0) {
            secs = 0;
        }
        secs = Math.floor(secs);
        var days = Math.floor(secs / 86400);
        secs %= 86400;
        var hours = Math.floor(secs / 3600);
        secs %= 3600;
        var minutes = Math.floor(secs / 60);
        var seconds = secs % 60;
        return formatCompactDurationParts(days, hours, minutes, seconds, maxParts).join(' ');
    }

    function formatLoadSummary(loadObj) {
        if (!loadObj || typeof loadObj !== 'object') {
            return '';
        }
        var pairs = [
            ['load1', 'word-loadavg1'],
            ['load5', 'word-loadavg5'],
            ['load15', 'word-loadavg15']
        ];
        return pairs
            .filter(function (pair) {
                return loadObj[pair[0]] !== undefined && loadObj[pair[0]] !== null && loadObj[pair[0]] !== '';
            })
            .map(function (pair) {
                return langTextSafe(pair[1]) + ': ' + loadObj[pair[0]];
            })
            .join(' / ');
    }

    function formatUnixTimestamp(value) {
        var unix = Number(value);
        if (!isFinite(unix) || unix <= 0) {
            return '';
        }
        var date = new Date(unix * 1000);
        if (isNaN(date.getTime())) {
            return '';
        }
        var year = date.getFullYear();
        var month = String(date.getMonth() + 1).padStart(2, '0');
        var day = String(date.getDate()).padStart(2, '0');
        var hours = String(date.getHours()).padStart(2, '0');
        var minutes = String(date.getMinutes()).padStart(2, '0');
        var seconds = String(date.getSeconds()).padStart(2, '0');
        return year + '-' + month + '-' + day + ' ' + hours + ':' + minutes + ':' + seconds;
    }

    function changeunit(limit) {
        var sign = limit < 0 ? "-" : "";
        var abs = Math.abs(limit);
        var size = "";
        if (abs < 0.1 * 1024) {
            size = abs.toFixed(2) + unitText('unit-byte');
        } else if (abs < 0.1 * 1024 * 1024) {
            size = (abs / 1024).toFixed(2) + unitText('unit-kilobyte');
        } else if (abs < 0.1 * 1024 * 1024 * 1024) {
            size = (abs / (1024 * 1024)).toFixed(2) + unitText('unit-megabyte');
        } else if (abs < 0.1 * 1024 * 1024 * 1024 * 1024) {
            size = (abs / (1024 * 1024 * 1024)).toFixed(2) + unitText('unit-gigabyte');
        } else {
            size = (abs / (1024 * 1024 * 1024 * 1024)).toFixed(2) + unitText('unit-terabyte');
        }
        var idx = size.indexOf(".");
        var dec = size.substr(idx + 1, 2);
        if (dec === "00") {
            size = size.substring(0, idx) + size.substr(idx + 3);
        }
        return sign + size;
    }

    function getRemainingTime(str) {
        if (str === '0001-01-01T00:00:00Z') {
            return {totalMs: Infinity, days: Infinity, hours: Infinity, minutes: Infinity, seconds: Infinity, formatted: '∞'};
        }
        var exp = new Date(str);
        var diff = exp - Date.now();
        if (diff <= 0) {
            return {totalMs: 0, days: 0, hours: 0, minutes: 0, seconds: 0, formatted: '0'};
        }
        var s = Math.floor(diff / 1e3);
        var days = Math.floor(s / 86400);
        var hours = Math.floor((s % 86400) / 3600);
        var minutes = Math.floor((s % 3600) / 60);
        var seconds = s % 60;
        return {
            totalMs: diff,
            days: days,
            hours: hours,
            minutes: minutes,
            seconds: seconds,
            formatted: formatCompactDurationParts(days, hours, minutes, seconds, 2).join(' ')
        };
    }

    function showMsg(text, type, dur, cb) {
        if (type === void 0) {
            type = 'success';
        }
        if (dur === void 0) {
            dur = 1500;
        }
        var old = document.getElementById('wangmarket_loading');
        if (old && old.parentNode) {
            old.parentNode.removeChild(old);
        }
        var isLong = text && text.length > 5;
        var svg;
        if (type === 'error') {
            svg = '<svg style="width:3rem;height:3rem;padding:1rem;box-sizing:content-box;" viewBox="0 0 1024 1024" xmlns="http://www.w3.org/2000/svg"><path d="M696.832 326.656c-12.8-12.8-33.28-12.8-46.08 0L512 465.92 373.248 327.168c-12.8-12.8-33.28-12.8-46.08 0s-12.8 33.28 0 46.08L466.432 512l-139.264 139.264c-12.8 12.8-12.8 33.28 0 46.08s33.28 12.8 46.08 0L512 558.08l138.752 139.264c12.288 12.8 32.768 12.8 45.568 0.512l0.512-0.512c12.8-12.8 12.8-33.28 0-45.568L557.568 512l139.264-139.264c12.8-12.8 12.8-33.28 0-46.08 0 0.512 0 0 0 0zM512 51.2c-254.464 0-460.8 206.336-460.8 460.8s206.336 460.8 460.8 460.8 460.8-206.336 460.8-460.8-206.336-460.8-460.8-460.8z m280.064 740.864c-74.24 74.24-175.104 116.224-280.064 115.712-104.96 0-205.824-41.472-280.064-115.712S115.712 616.96 115.712 512s41.472-205.824 116.224-280.064C306.176 157.696 407.04 115.712 512 116.224c104.96 0 205.824 41.472 280.064 116.224 74.24 74.24 116.224 175.104 115.712 280.064 0.512 104.448-41.472 205.312-115.712 279.552z" fill="#ffffff"></path></svg>';
        } else if (type === 'success') {
            svg = '<svg style="width:3rem;height:3rem;padding:1rem;box-sizing:content-box;" viewBox="0 0 1024 1024"><path d="M384 887.456L25.6 529.056 145.056 409.6 384 648.544 878.944 153.6 998.4 273.056z" fill="#ffffff"/></svg>';
        } else {
            svg = '<img style="width:3rem;height:3rem;padding:1rem;box-sizing:content-box;" src="data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAzMiAzMiIgd2lkdGg9IjY0IiBoZWlnaHQ9IjY0IiBmaWxsPSIjRjlGOUY5Ij4KICA8Y2lyY2xlIGN4PSIxNiIgY3k9IjMiIHI9IjAiPgogICAgPGFuaW1hdGUgYXR0cmlidXRlTmFtZT0iciIgdmFsdWVzPSIwOzM7MDswIiBkdXI9IjFzIiByZXBlYXRDb3VudD0iaW5kZWZpbml0ZSIgYmVnaW49IjAiIGtleVNwbGluZXM9IjAuMiAwLjIgMC40IDAuODswLjIgMC4yIDAuNCAwLjg7MC4yIDAuMiAwLjQgMC44IiBjYWxjTW9kZT0ic3BsaW5lIiAvPgogIDwvY2lyY2xlPgogIDxjaXJjbGUgdHJhbnNmb3JtPSJyb3RhdGUoNDUgMTYgMTYpIiBjeD0iMTYiIGN5PSIzIiByPSIwIj4KICAgIDxhbmltYXRlIGF0dHJpYnV0ZU5hbWU9InIiIHZhbHVlcz0iMDszOzA7MCIgZHVyPSIxcyIgcmVwZWF0Q291bnQ9ImluZGVmaW5pdGUiIGJlZ2luPSIwLjEyNXMiIGtleVNwbGluZXM9IjAuMiAwLjIgMC40IDAuODswLjIgMC4yIDAuNCAwLjg7MC4yIDAuMiAwLjQgMC44IiBjYWxjTW9kZT0ic3BsaW5lIiAvPgogIDwvY2lyY2xlPgogIDxjaXJjbGUgdHJhbnNmb3JtPSJyb3RhdGUoOTAgMTYgMTYpIiBjeD0iMTYiIGN5PSIzIiByPSIwIj4KICAgIDxhbmltYXRlIGF0dHJpYnV0ZU5hbWU9InIiIHZhbHVlcz0iMDszOzA7MCIgZHVyPSIxcyIgcmVwZWF0Q291bnQ9ImluZGVmaW5pdGUiIGJlZ2luPSIwLjI1cyIga2V5U3BsaW5lcz0iMC4yIDAuMiAwLjQgMC44OzAuMiAwLjIgMC40IDAuODswLjIgMC4yIDAuNCAwLjgiIGNhbGNNb2RlPSJzcGxpbmUiIC8+CiAgPC9jaXJjbGU+CiAgPGNpcmNsZSB0cmFuc2Zvcm09InJvdGF0ZSgxMzUgMTYgMTYpIiBjeD0iMTYiIGN5PSIzIiByPSIwIj4KICAgIDxhbmltYXRlIGF0dHJpYnV0ZU5hbWU9InIiIHZhbHVlcz0iMDszOzA7MCIgZHVyPSIxcyIgcmVwZWF0Q291bnQ9ImluZGVmaW5pdGUiIGJlZ2luPSIwLjM3NXMiIGtleVNwbGluZXM9IjAuMiAwLjIgMC40IDAuODswLjIgMC4yIDAuNCAwLjg7MC4yIDAuMiAwLjQgMC44IiBjYWxjTW9kZT0ic3BsaW5lIiAvPgogIDwvY2lyY2xlPgogIDxjaXJjbGUgdHJhbnNmb3JtPSJyb3RhdGUoMTgwIDE2IDE2KSIgY3g9IjE2IiBjeT0iMyIgcj0iMCI+CiAgICA8YW5pbWF0ZSBhdHRyaWJ1dGVOYW1lPSJyIiB2YWx1ZXM9IjA7MzswOzAiIGR1cj0iMXMiIHJlcGVhdENvdW50PSJpbmRlZmluaXRlIiBiZWdpbj0iMC41cyIga2V5U3BsaW5lcz0iMC4yIDAuMiAwLjQgMC44OzAuMiAwLjIgMC40IDAuODswLjIgMC4yIDAuNCAwLjgiIGNhbGNNb2RlPSJzcGxpbmUiIC8+CiAgPC9jaXJjbGU+CiAgPGNpcmNsZSB0cmFuc2Zvcm09InJvdGF0ZSgyMjUgMTYgMTYpIiBjeD0iMTYiIGN5PSIzIiByPSIwIj4KICAgIDxhbmltYXRlIGF0dHJpYnV0ZU5hbWU9InIiIHZhbHVlcz0iMDszOzA7MCIgZHVyPSIxcyIgcmVwZWF0Q291bnQ9ImluZGVmaW5pdGUiIGJlZ2luPSIwLjYyNXMiIGtleVNwbGluZXM9IjAuMiAwLjIgMC40IDAuODswLjIgMC4yIDAuNCAwLjg7MC4yIDAuMiAwLjQgMC44IiBjYWxjTW9kZT0ic3BsaW5lIiAvPgogIDwvY2lyY2xlPgogIDxjaXJjbGUgdHJhbnNmb3JtPSJyb3RhdGUoMjcwIDE2IDE2KSIgY3g9IjE2IiBjeT0iMyIgcj0iMCI+CiAgICA8YW5pbWF0ZSBhdHRyaWJ1dGVOYW1lPSJyIiB2YWx1ZXM9IjA7MzswOzAiIGR1cj0iMXMiIHJlcGVhdENvdW50PSJpbmRlZmluaXRlIiBiZWdpbj0iMC43NXMiIGtleVNwbGluZXM9IjAuMiAwLjIgMC40IDAuODswLjIgMC4yIDAuNCAwLjg7MC4yIDAuMiAwLjQgMC44IiBjYWxjTW9kZT0ic3BsaW5lIiAvPgogIDwvY2lyY2xlPgogIDxjaXJjbGUgdHJhbnNmb3JtPSJyb3RhdGUoMzE1IDE2IDE2KSIgY3g9IjE2IiBjeT0iMyIgcj0iMCI+CiAgICA8YW5pbWF0ZSBhdHRyaWJ1dGVOYW1lPSJyIiB2YWx1ZXM9IjA7MzswOzAiIGR1cj0iMXMiIHJlcGVhdENvdW50PSJpbmRlZmluaXRlIiBiZWdpbj0iMC44NzVzIiBrZXlTcGxpbmVzPSIwLjIgMC4yIDAuNCAwLjg7MC4yIDAuMiAwLjQgMC44OzAuMiAwLjIgMC40IDAuOCIgY2FsY01vZGU9InNwbGluZSIgLz4KICA8L2NpcmNsZT4KICA8Y2lyY2xlIHRyYW5zZm9ybT0icm90YXRlKDE4MCAxNiAxNikiIGN4PSIxNiIgY3k9IjMiIHI9IjAiPgogICAgPGFuaW1hdGUgYXR0cmlidXRlTmFtZT0iciIgdmFsdWVzPSIwOzM7MDswIiBkdXI9IjFzIiByZXBlYXRDb3VudD0iaW5kZWZpbml0ZSIgYmVnaW49IjAuNXMiIGtleVNwbGluZXM9IjAuMiAwLjIgMC40IDAuODswLjIgMC4yIDAuNCAwLjg7MC4yIDAuMiAwLjQgMC44IiBjYWxjTW9kZT0ic3BsaW5lIiAvPgogIDwvY2lyY2xlPgo8L3N2Zz4K" />';
        }
        var clickable = (type === 'error' || type === 'success');
        var w = document.createElement('div');
        w.id = 'wangmarket_loading';
        w.style = 'position:fixed;top:0;z-index: 2147483647;width:100%;height:100%;display:flex;flex-direction:column;justify-content:center;align-items:center';
        w.innerHTML =
          '<div id="loading">'
            + '<div id="loading_box" style="background-color:#2e2d3c;border-radius:0.3rem;opacity:0.8;min-width:6rem;min-height:4.8rem;max-width:20rem;display:flex;flex-wrap:wrap;align-items:center;'
              + (!isLong ? 'flex-direction:column;' : '') + (clickable ? 'cursor:pointer;' : '') + '">'
              + '<div style="display:flex;">' + svg + '</div>'
              + '<div style="font-size:1rem;box-sizing:border-box;color:white;flex:1;padding:1rem;' + (!isLong ? 'padding-top:0;' : '') + '">'
                + text
              + '</div>'
            + '</div>'
          + '</div>';
        document.body.appendChild(w);
        var timer = setTimeout(function () {
            if (w && w.parentNode) {
                w.parentNode.removeChild(w);
            }
            if (typeof cb === 'function') {
                cb();
            }
        }, dur);
        if (clickable) {
            var box = document.getElementById('loading_box');
            if (box) {
                box.addEventListener('click', function () {
                    clearTimeout(timer);
                    if (w && w.parentNode) {
                        w.parentNode.removeChild(w);
                    }
                    if (typeof cb === 'function') {
                        cb();
                    }
                }, { once: true, passive: true });
            }
        }
    }

    function oCopy(obj) {
        var tempInput = document.createElement("input");
        document.body.appendChild(tempInput);
        tempInput.value = obj.innerText || obj.textContent;
        tempInput.select();
        document.execCommand('copy');
        document.body.removeChild(tempInput);
        showMsg(feedbackLangText('reply.copied', ''));
    }

    function copyText(text) {
        var textarea = document.createElement("textarea");
        textarea.value = text;
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand("copy");
        document.body.removeChild(textarea);
        showMsg(feedbackLangText('reply.copied', ''));
    }

    function escapeHtml(str) {
        return String(str).replace(/[&<>\"']/g, function (s) {
            return ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'})[s];
        });
    }

    function getBridgeMode(data) {
        if (typeof data !== 'string') {
            return '';
        }
        var parts = data.split(',', 2);
        var first = parts[0] || '';
        var second = parts[1] || '';
        var escapedFirst = escapeHtml(first).toUpperCase();
        var escapedSecond = escapeHtml(second).toUpperCase();
        if (!second || first === second) {
            return escapedFirst;
        }
        return escapedSecond + ' → ' + escapedFirst;
    }

    window.changeunit = changeunit;
    window.getRemainingTime = getRemainingTime;
    window.showMsg = showMsg;
    window.oCopy = oCopy;
    window.copyText = copyText;
    window.escapeHtml = escapeHtml;
    window.getBridgeMode = getBridgeMode;
    window.formatCompactDuration = formatCompactDuration;
    window.formatLoadSummary = formatLoadSummary;
    window.formatUnixTimestamp = formatUnixTimestamp;

    var nps = window.nps;
    nps.changeunit = changeunit;
    nps.getRemainingTime = getRemainingTime;
    nps.showMsg = showMsg;
    nps.copyText = copyText;
    nps.escapeHtml = escapeHtml;
    nps.getBridgeMode = getBridgeMode;
    nps.formatCompactDuration = formatCompactDuration;
    nps.formatLoadSummary = formatLoadSummary;
    nps.formatUnixTimestamp = formatUnixTimestamp;
})();
