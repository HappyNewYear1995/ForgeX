// Auto-refresh build status every 10 seconds
(function() {
    'use strict';

    // Highlight active nav link
    document.querySelectorAll('.nav-link').forEach(function(link) {
        var href = link.getAttribute('href');
        if (href && window.location.pathname.startsWith(href)) {
            link.classList.add('active');
        }
    });

    // Toast notification
    function showToast(msg, duration) {
        var toast = document.getElementById('toast');
        if (!toast) return;
        toast.textContent = msg;
        toast.style.display = 'block';
        setTimeout(function() { toast.style.display = 'none'; }, duration || 3000);
    }
    window.showToast = showToast;

    // Check URL for saved parameter
    var params = new URLSearchParams(window.location.search);
    var saved = params.get('saved');
    if (saved) {
        var names = { jenkins: 'Jenkins', gitea: 'Gitea' };
        var displayName = names[saved] || saved;
        showToast('✅ ' + displayName + ' 配置保存成功', 3000);
        // Clean URL
        history.replaceState(null, '', window.location.pathname);
    }

    // Test API connectivity
    window.testAPI = function(btn, category) {
        var container = document.getElementById('apiStatus_' + category);
        var statusEl = container ? container.querySelector('.api-status') : null;
        var timeEl = container ? container.querySelector('.sys-test-time') : null;
        btn.disabled = true;
        btn.textContent = '测试中...';
        if (statusEl) { statusEl.className = 'api-status api-testing'; statusEl.textContent = '连接中...'; }

        // Step 1: Save current form values first
        var form = document.querySelector('.sys-config-form[data-category="' + category + '"]');
        var savePromise = Promise.resolve();
        if (form) {
            var formData = new URLSearchParams(new FormData(form));
            savePromise = fetch('/config/sys/' + category, {
                method: 'POST',
                headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
                body: formData.toString()
            }).then(function(r) {
                if (!r.ok) { return r.text().then(function(t) { throw new Error(t || '保存失败'); }); }
            });
        }

        // Step 2: Test connection after save
        savePromise.then(function() {
            return fetch('/config/sys/' + category + '/test', { method: 'POST' });
        })
        .then(function(r) { return r.json(); })
        .then(function(data) {
            var now = new Date();
            var timeStr = now.getFullYear() + '-' +
                String(now.getMonth()+1).padStart(2,'0') + '-' +
                String(now.getDate()).padStart(2,'0') + ' ' +
                String(now.getHours()).padStart(2,'0') + ':' +
                String(now.getMinutes()).padStart(2,'0') + ':' +
                String(now.getSeconds()).padStart(2,'0');
            if (data.ok) {
                if (statusEl) {
                    statusEl.className = 'api-status api-ok';
                    statusEl.textContent = '✓ 连接正常';
                }
                if (container) { container.setAttribute('data-status', 'ok'); container.setAttribute('data-time', timeStr); }
                if (timeEl) { timeEl.textContent = '最近测试: ' + timeStr; } else if (container && !timeEl) { var s = document.createElement('span'); s.className = 'sys-test-time'; s.textContent = '最近测试: ' + timeStr; container.appendChild(s); }
                showToast('✅ ' + (category === 'jenkins' ? 'Jenkins' : 'Gitea') + ' 连接正常', 2500);
            } else {
                if (statusEl) {
                    statusEl.className = 'api-status api-err';
                    statusEl.textContent = '✗ 连接异常';
                }
                if (container) { container.setAttribute('data-status', 'err'); container.setAttribute('data-time', timeStr); }
                if (timeEl) { timeEl.textContent = '最近测试: ' + timeStr; } else if (container && !timeEl) { var s = document.createElement('span'); s.className = 'sys-test-time'; s.textContent = '最近测试: ' + timeStr; container.appendChild(s); }
                showToast('❌ ' + (data.error || '连接失败'), 4000);
            }
        })
        .catch(function(e) {
            if (statusEl) { statusEl.className = 'api-status api-err'; statusEl.textContent = '✗ 连接异常'; }
            showToast('❌ ' + e.message, 4000);
        })
        .finally(function() {
            btn.disabled = false;
            btn.textContent = '测试连接';
        });
    };

    // Refresh page if there are running builds
    function checkRunningBuilds() {
        var badges = document.querySelectorAll('.badge-running, .badge-pending');
        if (badges.length > 0) {
            setTimeout(function() {
                window.location.reload();
            }, 10000);
        }
    }

    // Config tree loader for build form
    function initConfigTree(productId) {
        var box = document.getElementById('configTreeBox');
        if (!box) return;
        var url = '/config/tree/json';
        if (productId) url += '?product_id=' + productId;
        fetch(url)
            .then(function(r) { return r.json(); })
            .then(function(tree) {
                if (!tree || tree.length === 0) {
                    box.innerHTML = '<div class="config-empty-msg">暂无配置，请先在「配置管理」中添加组件和模块</div>';
                    return;
                }
                var html = '';
                tree.forEach(function(comp) {
                    var mods = comp.children || [];
                    var gitUrl = comp.git_url || '';
                    var branchFilter = comp.branch_filter || '';
                    html += '<div class="config-comp" data-comp-id="' + comp.id + '" data-comp-code="' + comp.code + '" data-git-url="' + gitUrl + '" data-branch-filter="' + branchFilter + '">';
                    html += '<label class="config-comp-header">';
                    html += '<input type="checkbox" class="comp-check" value="' + comp.id + '"> ';
                    html += '<span>' + comp.name + '</span>';
                    html += '<span class="config-comp-code">(' + comp.code + ')</span>';
                    if (mods.length > 0) html += ' <span style="font-size:0.75rem;color:#94a3b8">- ' + mods.length + '个模块</span>';
                    if (mods.length > 0) html += ' <a href="javascript:void(0)" class="mod-select-all" onclick="toggleAllMods(this)">全选</a>';
                    html += '</label>';
                    // Branch selector (hidden until component is checked)
                    html += '<div class="config-branch-select" style="display:none">';
                    html += '<label>分支: </label>';
                    html += '<select class="branch-select" name="comp_branch_' + comp.id + '"><option value="">加载中...</option></select>';
                    html += '</div>';
                    if (mods.length > 0) {
                        html += '<div class="config-modules">';
                        mods.forEach(function(mod) {
                            html += '<label class="config-mod-chip">';
                            html += '<input type="checkbox" class="mod-check" value="' + mod.id + '" data-code="' + mod.code + '"> ';
                            html += mod.name;
                            html += '</label>';
                        });
                        html += '</div>';
                    }
                    html += '</div>';
                });
                box.innerHTML = html;

                // Toggle modules visibility and branch loading when component is checked
                box.querySelectorAll('.comp-check').forEach(function(cb) {
                    cb.addEventListener('change', function() {
                        var comp = cb.closest('.config-comp');
                        var mods = comp.querySelector('.config-modules');
                        var branchDiv = comp.querySelector('.config-branch-select');
                        if (mods) {
                            if (cb.checked) mods.classList.add('open');
                            else mods.classList.remove('open');
                        }
                        // Show/hide branch selector and load branches
                        if (branchDiv) {
                            branchDiv.style.display = cb.checked ? '' : 'none';
                            if (cb.checked) loadBranches(comp);
                        }
                        updateComponentMods();
                    });
                });
                box.querySelectorAll('.mod-check').forEach(function(cb) {
                    cb.addEventListener('change', function() {
                        cb.closest('.config-mod-chip').classList.toggle('selected', cb.checked);
                        // Sync "select all" link text
                        var comp = cb.closest('.config-comp');
                        var link = comp.querySelector('.mod-select-all');
                        if (link) {
                            var all = comp.querySelectorAll('.mod-check');
                            var allChecked = all.length > 0;
                            all.forEach(function(c) { if (!c.checked) allChecked = false; });
                            link.textContent = allChecked ? '取消全选' : '全选';
                        }
                        updateComponentMods();
                    });
                });
                // Update hidden field when branch selection changes
                box.querySelectorAll('.branch-select').forEach(function(sel) {
                    sel.addEventListener('change', function() {
                        updateComponentMods();
                    });
                });
            })
            .catch(function() {
                box.innerHTML = '<div class="config-empty-msg">加载配置失败</div>';
            });
    }

    // Load branches for a component from Gitea API
    function loadBranches(compDiv) {
        var gitUrl = compDiv.getAttribute('data-git-url');
        var branchFilter = compDiv.getAttribute('data-branch-filter');
        var select = compDiv.querySelector('.branch-select');
        if (!select || !gitUrl) {
            if (select) select.innerHTML = '<option value="">未配置Git仓库</option>';
            return;
        }
        select.innerHTML = '<option value="">加载中...</option>';
        fetch('/api/gitea/branches?git_url=' + encodeURIComponent(gitUrl))
            .then(function(r) { return r.json(); })
            .then(function(branches) {
                if (!branches || branches.length === 0) {
                    select.innerHTML = '<option value="">无分支</option>';
                    return;
                }
                var html = '';
                var filter = branchFilter;
                branches.forEach(function(b) {
                    // Apply branch filter if configured
                    if (filter && filter.trim() && !matchBranch(b, filter)) return;
                    var selected = (filter && b === filter) ? ' selected' : '';
                    html += '<option value="' + b + '"' + selected + '>' + b + '</option>';
                });
                if (!html) html = '<option value="">无匹配分支</option>';
                select.innerHTML = html;
            })
            .catch(function() {
                select.innerHTML = '<option value="">加载失败</option>';
            });
    }

    function matchBranch(branchName, filter) {
        if (!filter || filter.trim() === '') return true;
        // Support comma-separated patterns and wildcard *
        var patterns = filter.split(',');
        for (var i = 0; i < patterns.length; i++) {
            var p = patterns[i].trim();
            if (p === '') continue;
            if (p === '*') return true;
            if (p.indexOf('*') >= 0) {
                var re = new RegExp('^' + p.replace(/\*/g, '.*') + '$');
                if (re.test(branchName)) return true;
            } else if (branchName === p) {
                return true;
            }
        }
        return false;
    }

    function updateComponentMods() {
        var box = document.getElementById('configTreeBox');
        var hidden = document.getElementById('componentMods');
        if (!box || !hidden) return;
        var result = [];
        box.querySelectorAll('.config-comp').forEach(function(comp) {
            var cb = comp.querySelector('.comp-check');
            if (!cb.checked) return;
            var modules = [];
            comp.querySelectorAll('.mod-check:checked').forEach(function(mcb) {
                modules.push({ id: parseInt(mcb.value), code: mcb.getAttribute('data-code') });
            });
            var branchSelect = comp.querySelector('.branch-select');
            var branch = branchSelect ? branchSelect.value : '';
            result.push({
                component_id: parseInt(cb.value),
                component_code: comp.getAttribute('data-comp-code'),
                branch: branch,
                modules: modules
            });
        });
        hidden.value = JSON.stringify(result);
    }

    // Expandable table rows
    function initExpandRows() {
        document.querySelectorAll('.expand-btn').forEach(function(btn) {
            btn.addEventListener('click', function(e) {
                e.stopPropagation();
                var row = btn.closest('.row-main');
                var pid = row.getAttribute('data-product-id');
                var expandRow = document.querySelector('.row-expand[data-product-id="' + pid + '"]');
                if (!expandRow) return;

                var isOpen = expandRow.style.display !== 'none';
                if (isOpen) {
                    expandRow.style.display = 'none';
                    btn.classList.remove('active');
                    return;
                }

                // Open
                expandRow.style.display = '';
                btn.classList.add('active');

                // Only fetch once
                if (expandRow.getAttribute('data-loaded')) return;
                expandRow.setAttribute('data-loaded', '1');

                var loading = expandRow.querySelector('.expand-loading');
                var empty = expandRow.querySelector('.expand-empty');
                var table = expandRow.querySelector('.table-nested');
                loading.style.display = 'flex';

                fetch('/products/' + pid + '/releases/json')
                    .then(function(r) { return r.json(); })
                    .then(function(releases) {
                        loading.style.display = 'none';
                        if (!releases || releases.length === 0) {
                            empty.style.display = '';
                            return;
                        }
                        var tbody = table.querySelector('tbody');
                        releases.forEach(function(rel) {
                            var tr = document.createElement('tr');
                            var date = rel.created_at ? new Date(rel.created_at).toLocaleString('zh-CN', {year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'}) : '-';
                            tr.innerHTML =
                                '<td><strong>' + (rel.version||'') + '</strong></td>' +
                                '<td><span class="badge badge-env-' + (rel.build_env||'') + '">' + (rel.build_env||'') + '</span></td>' +
                                '<td><span class="badge badge-release-' + (rel.status||'') + '">' + (rel.status||'') + '</span></td>' +
                                '<td>' + (rel.description||'-') + '</td>' +
                                '<td>' + date + '</td>' +
                                '<td><a href="/releases/' + rel.id + '" class="btn btn-sm">Manifest</a></td>';
                            tbody.appendChild(tr);
                        });
                        table.style.display = '';
                    })
                    .catch(function() {
                        loading.style.display = 'none';
                        empty.textContent = '加载失败';
                        empty.style.display = '';
                    });
            });
        });
    }

    // Component selection toggle (product form)
    window.toggleCompConfig = function(checkbox, compId) {
        var configDiv = document.getElementById('compConfig_' + compId);
        if (configDiv) {
            configDiv.style.display = checkbox.checked ? '' : 'none';
        }
    };
    // Toggle all modules for a component (build modal)
    window.toggleAllMods = function(link) {
        var comp = link.closest('.config-comp');
        var checks = comp.querySelectorAll('.mod-check');
        var allChecked = true;
        checks.forEach(function(cb) { if (!cb.checked) allChecked = false; });
        var newVal = !allChecked;
        checks.forEach(function(cb) {
            cb.checked = newVal;
            cb.closest('.config-mod-chip').classList.toggle('selected', newVal);
        });
        link.textContent = newVal ? '取消全选' : '全选';
        updateComponentMods();
    };
    // Test environment toggle (product form)
    window.toggleTestEnv = function() {
        var checked = document.getElementById('testEnvEnabled').checked;
        document.getElementById('testEnvFields').style.display = checked ? '' : 'none';
    };

    // Product form validation
    window.validateProductForm = function(form) {
        var compChecks = form.querySelectorAll('input[name="comp_ids"]:checked');
        if (compChecks.length === 0) {
            alert('请至少选择一个组件');
            return false;
        }
        for (var i = 0; i < compChecks.length; i++) {
            var id = compChecks[i].value;
            var gitInput = form.querySelector('input[name="comp_git_url_' + id + '"]');
            if (!gitInput || !gitInput.value.trim()) {
                var label = compChecks[i].closest('label');
                var name = label ? label.querySelector('strong')?.textContent || '组件' : '组件';
                alert('组件「' + name + '」必须填写Git仓库地址');
                gitInput && gitInput.focus();
                return false;
            }
        }
        var testEnvCb = document.getElementById('testEnvEnabled');
        if (testEnvCb && testEnvCb.checked) {
            var envChecks = form.querySelectorAll('input[name="test_env_ids"]:checked');
            if (envChecks.length === 0) {
                alert('启用测试环境时，请至少选择一个测试环境');
                return false;
            }
        }
        return true;
    };

    // Repo search dropdown (product form)
    var repoCache = {};
    var repoTimer = null;

    function fetchRepos(query, callback) {
        clearTimeout(repoTimer);
        repoTimer = setTimeout(function() {
            var url = '/api/gitea/repos' + (query ? '?q=' + encodeURIComponent(query) : '');
            fetch(url)
                .then(function(r) { return r.json(); })
                .then(function(data) { callback(data); })
                .catch(function() { callback([]); });
        }, 300);
    }

    function renderDropdown(dropdown, repos, input, hiddenInput) {
        if (!repos || repos.length === 0) {
            dropdown.innerHTML = '<div class="repo-dropdown-empty">无匹配仓库</div>';
            dropdown.style.display = '';
            return;
        }
        var html = '';
        repos.forEach(function(repo) {
            html += '<div class="repo-dropdown-item" data-url="' + repo.clone_url + '" data-name="' + repo.full_name + '">'
                + '<span class="repo-name">' + repo.full_name + '</span>'
                + '</div>';
        });
        dropdown.innerHTML = html;
        dropdown.style.display = '';

        dropdown.querySelectorAll('.repo-dropdown-item').forEach(function(item) {
            item.addEventListener('mousedown', function(e) {
                e.preventDefault();
                input.value = item.getAttribute('data-name');
                hiddenInput.value = item.getAttribute('data-url');
                dropdown.style.display = 'none';
                input.classList.add('repo-selected');
            });
        });
    }

    document.querySelectorAll('.repo-search-input').forEach(function(input) {
        var wrapper = input.closest('.repo-select-wrapper');
        var dropdown = wrapper.querySelector('.repo-dropdown');
        var hiddenInput = wrapper.querySelector('.repo-url-value');

        input.addEventListener('focus', function() {
            if (input.value === '') {
                fetchRepos('', function(repos) { renderDropdown(dropdown, repos, input, hiddenInput); });
            }
        });

        input.addEventListener('input', function() {
            hiddenInput.value = '';
            input.classList.remove('repo-selected');
            var q = input.value.trim();
            fetchRepos(q, function(repos) { renderDropdown(dropdown, repos, input, hiddenInput); });
        });

        input.addEventListener('blur', function() {
            setTimeout(function() { dropdown.style.display = 'none'; }, 150);
        });
    });

    // Build modal
    window.openBuildModal = function(productId, productName, currentVersion) {
        var modal = document.getElementById('buildModal');
        var form = document.getElementById('buildForm');
        var title = document.getElementById('buildModalTitle');
        if (!modal) return;
        form.action = '/products/' + productId + '/build';
        title.textContent = '触发构建 - ' + productName;
        // Reset form state
        form.reset();
        document.getElementById('specifyVersion').checked = false;
        document.getElementById('isFormal').checked = false;
        document.getElementById('versionInputGroup').style.display = 'none';
        document.getElementById('releaseNotesGroup').style.display = 'none';
        document.querySelector('input[name="auto_sync_test"]').checked = true;
        document.querySelector('input[name="build_type"][value="upgrade"]').checked = true;
        // Pre-fill version as default value (hidden until checkbox is ticked)
        var versionInput = document.getElementById('buildVersion');
        versionInput.value = '';
        versionInput.disabled = true;
        modal.style.display = 'flex';
        // Load config tree
        initConfigTree(productId);
    };
    window.closeBuildModal = function() {
        var modal = document.getElementById('buildModal');
        if (modal) modal.style.display = 'none';
    };
    window.toggleVersionInput = function() {
        var checked = document.getElementById('specifyVersion').checked;
        var input = document.getElementById('buildVersion');
        document.getElementById('versionInputGroup').style.display = checked ? '' : 'none';
        if (!checked) {
            input.value = '';
            input.disabled = true;
        } else {
            input.disabled = false;
        }
    };
    window.toggleReleaseNotes = function() {
        var checked = document.getElementById('isFormal').checked;
        document.getElementById('releaseNotesGroup').style.display = checked ? '' : 'none';
    };
    // Close build modal on backdrop click
    (function() {
        var bm = document.getElementById('buildModal');
        if (bm) {
            bm.addEventListener('click', function(e) {
                if (e.target === bm) closeBuildModal();
            });
        }
    })();

    // Card collapse/expand
    window.toggleCollapse = function(header) {
        var card = header.closest('.card');
        if (!card) return;
        var id = card.getAttribute('data-collapse-id');
        var collapsed = card.classList.toggle('collapsed');
        var textEl = header.querySelector('.collapse-text');
        if (textEl) textEl.textContent = collapsed ? '展开' : '缩起';
        if (id) {
            try { localStorage.setItem('collapse_' + id, collapsed ? '1' : '0'); } catch(e) {}
        }
    };

    function initCollapse() {
        document.querySelectorAll('.card[data-collapse-id]').forEach(function(card) {
            var id = card.getAttribute('data-collapse-id');
            try {
                if (localStorage.getItem('collapse_' + id) === '1') {
                    card.classList.add('collapsed');
                    var textEl = card.querySelector('.collapse-text');
                    if (textEl) textEl.textContent = '展开';
                }
            } catch(e) {}
        });
    }

    // Inline edit toggle for config page
    window.toggleEdit = function(btn) {
        var tr = btn.closest('tr');
        var id = tr.getAttribute('data-id');
        // Find sibling rows with same data-id
        var viewRow = tr.closest('table').querySelector('tr.row-view[data-id="' + id + '"]');
        var editRow = tr.closest('table').querySelector('tr.row-edit[data-id="' + id + '"]');
        if (!viewRow || !editRow) return;
        var isEditing = editRow.style.display !== 'none';
        if (isEditing) {
            editRow.style.display = 'none';
            viewRow.style.display = '';
        } else {
            viewRow.style.display = 'none';
            editRow.style.display = '';
        }
    };

    // Confirm modal logic
    function initConfirmModal() {
        var modal = document.getElementById('confirmModal');
        var msgEl = document.getElementById('confirmModalMsg');
        var cancelBtn = document.getElementById('confirmCancel');
        var okBtn = document.getElementById('confirmOk');
        if (!modal) return;

        function closeModal() {
            modal.style.display = 'none';
            modal.removeAttribute('data-pending-action');
            modal.removeAttribute('data-pending-url');
        }

        cancelBtn.addEventListener('click', closeModal);
        modal.addEventListener('click', function(e) {
            if (e.target === modal) closeModal();
        });

        okBtn.addEventListener('click', function(e) {
            e.stopPropagation();
            var actionType = modal.getAttribute('data-pending-action');
            var actionUrl = modal.getAttribute('data-pending-url');
            closeModal();
            if (actionType === 'form' && actionUrl) {
                fetch(actionUrl, { method: 'POST', redirect: 'follow' })
                    .then(function() { window.location.reload(); })
                    .catch(function() { window.location.reload(); });
            }
        });

        // Intercept forms with data-confirm attribute
        document.addEventListener('submit', function(e) {
            var form = e.target;
            var msg = form.getAttribute('data-confirm');
            if (!msg) return;
            e.preventDefault();
            msgEl.textContent = msg;
            modal.setAttribute('data-pending-action', 'form');
            modal.setAttribute('data-pending-url', form.action);
            modal.style.display = 'flex';
        }, true);

        // Intercept links/buttons with data-confirm attribute (not inside a data-confirm form)
        document.addEventListener('click', function(e) {
            // Skip if click is inside a form that has data-confirm (the submit handler will handle it)
            if (e.target.closest('form[data-confirm]')) return;
            var el = e.target.closest('[data-confirm]');
            if (!el) return;
            var msg = el.getAttribute('data-confirm');
            if (!msg) return;
            e.preventDefault();
            e.stopPropagation();
            msgEl.textContent = msg;
            modal.setAttribute('data-pending-action', 'click');
            modal.style.display = 'flex';
        }, true);
    }

    // ========================================
    // Script Management Modal (Playwright)
    // ========================================
    var smEnvId = null;
    var smData = {};

    // Load script data from embedded JSON
    (function() {
        var el = document.getElementById('scriptData');
        if (el) { try { smData = JSON.parse(el.textContent); } catch(e) {} }
    })();

    window.openScriptModal = function(envId, envName) {
        smEnvId = envId;
        var d = smData[envId] || {};
        document.getElementById('smEnvName').textContent = envName;
        // status
        var labels = { idle: '未执行', running: '执行中...', success: '执行成功', failed: '执行失败' };
        var badge = document.getElementById('smStatus');
        badge.className = 'script-status-badge script-status-' + (d.status || 'idle');
        badge.textContent = labels[d.status] || '未执行';
        document.getElementById('smTime').textContent = d.time || '';
        // preview
        var code = d.content || '';
        document.getElementById('smPreviewCode').textContent = code || '暂无脚本内容';
        document.getElementById('smPreview').style.display = '';
        document.getElementById('smEditor').style.display = 'none';
        document.getElementById('smRecording').style.display = 'none';
        document.getElementById('smTextarea').value = code;
        // log
        if (d.output) {
            document.getElementById('smLog').textContent = d.output;
            document.getElementById('smLogPanel').style.display = '';
        } else {
            document.getElementById('smLogPanel').style.display = 'none';
        }
        document.getElementById('scriptModal').style.display = 'flex';
    };

    window.closeScriptModal = function() {
        document.getElementById('scriptModal').style.display = 'none';
        smEnvId = null;
    };

    window.startRecording = function() {
        if (!smEnvId) return;
        if (!confirm('即将启动 Playwright 录制浏览器。\n\n操作步骤：\n1. 在浏览器中完成升级操作\n2. 点击 Playwright Inspector 窗口的「录制」按钮停止录制\n3. 关闭浏览器窗口\n\n⚠ 必须先点击「录制」按钮停止录制，否则不会生成脚本文件。')) return;
        document.getElementById('smPreview').style.display = 'none';
        document.getElementById('smEditor').style.display = 'none';
        document.getElementById('smRecording').style.display = 'flex';

        fetch('/config/testenv/' + smEnvId + '/script/record', { method: 'POST' })
            .then(function(r) { if (!r.ok) return r.text().then(function(t) { throw new Error(t); }); return r.json(); })
            .then(function() { pollRecording(); })
            .catch(function(err) {
                document.getElementById('smRecording').style.display = 'none';
                document.getElementById('smPreview').style.display = '';
                showToast('录制失败: ' + err.message, 'error');
            });
    };

    function pollRecording() {
        var iv = setInterval(function() {
            fetch('/config/testenv/' + smEnvId + '/script/record/status')
                .then(function(r) { return r.json(); })
                .then(function(d) {
                    if (!d.done) return;
                    clearInterval(iv);
                    document.getElementById('smRecording').style.display = 'none';
                    document.getElementById('smPreview').style.display = '';
                    if (d.error) {
                        showToast('录制失败: ' + d.error, 'error');
                    } else {
                        var code = d.content || '';
                        document.getElementById('smPreviewCode').textContent = code || '暂无脚本内容';
                        document.getElementById('smTextarea').value = code;
                        if (smData[smEnvId]) smData[smEnvId].content = code;
                        showToast('录制完成，脚本已保存', 'success');
                    }
                })
                .catch(function(err) { clearInterval(iv); showToast('轮询失败: ' + err.message, 'error'); });
        }, 2000);
    }

    window.toggleSmEdit = function() {
        var editor = document.getElementById('smEditor');
        var preview = document.getElementById('smPreview');
        var isEdit = editor.style.display !== 'none';
        editor.style.display = isEdit ? 'none' : 'block';
        preview.style.display = isEdit ? '' : 'none';
        if (!isEdit) document.getElementById('smTextarea').value = (smData[smEnvId] || {}).content || '';
    };

    window.saveScript = function() {
        if (!smEnvId) return;
        var content = document.getElementById('smTextarea').value;
        var params = new URLSearchParams();
        params.set('script_content', content);
        fetch('/config/testenv/' + smEnvId + '/script/save', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: params.toString()
        })
        .then(function(r) { return r.json(); })
        .then(function(d) {
            if (d.error) { showToast(d.error, 'error'); return; }
            if (!smData[smEnvId]) smData[smEnvId] = {};
            smData[smEnvId].content = content;
            document.getElementById('smPreviewCode').textContent = content || '暂无脚本内容';
            toggleSmEdit();
            showToast('保存成功', 'success');
        })
        .catch(function(err) { showToast('保存失败: ' + err.message, 'error'); });
    };

    window.runScriptExec = function() {
        if (!smEnvId) return;
        var btn = document.querySelector('.script-modal-actions .btn-run');
        btn.disabled = true; btn.textContent = '执行中...';
        var badge = document.getElementById('smStatus');
        badge.className = 'script-status-badge script-status-running';
        badge.textContent = '执行中...';
        document.getElementById('smLog').textContent = '正在执行脚本...';
        document.getElementById('smLogPanel').style.display = '';

        fetch('/config/testenv/' + smEnvId + '/script/run', { method: 'POST' })
            .then(function(r) { return r.json(); })
            .then(function() { pollExecStatus(); })
            .catch(function(err) {
                btn.disabled = false; btn.textContent = '执行';
                showToast('启动失败: ' + err.message, 'error');
            });
    };

    function pollExecStatus() {
        var iv = setInterval(function() {
            fetch('/config/testenv/' + smEnvId + '/script/output')
                .then(function(r) { return r.json(); })
                .then(function(d) {
                    document.getElementById('smLog').textContent = d.output || '(无输出)';
                    if (d.status !== 'running') {
                        clearInterval(iv);
                        var btn = document.querySelector('.script-modal-actions .btn-run');
                        btn.disabled = false; btn.textContent = '执行';
                        var labels = { idle: '未执行', success: '执行成功', failed: '执行失败' };
                        var badge = document.getElementById('smStatus');
                        badge.className = 'script-status-badge script-status-' + d.status;
                        badge.textContent = labels[d.status] || d.status;
                        if (smData[smEnvId]) {
                            smData[smEnvId].status = d.status;
                            smData[smEnvId].output = d.output;
                            smData[smEnvId].time = d.last_run || '';
                        }
                        document.getElementById('smTime').textContent = d.last_run || '';
                    }
                })
                .catch(function(err) { clearInterval(iv); showToast('轮询失败: ' + err.message, 'error'); });
        }, 2000);
    }

    // Build history script log toggle
    window.showScriptLog = function(buildId) {
        var row = document.getElementById('script-log-' + buildId);
        if (!row) return;
        row.style.display = row.style.display === 'none' ? 'table-row' : 'none';
    };

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            checkRunningBuilds();
            initConfirmModal();
            initExpandRows();
            initConfigTree();
            initCollapse();
        });
    } else {
        checkRunningBuilds();
        initConfirmModal();
        initExpandRows();
        initConfigTree();
        initCollapse();
    }
})();
