// This browser demo uses illustrative hosts and never opens a connection.
(() => {
  const search = document.querySelector('#host-search');
  const rows = [...document.querySelectorAll('.host-row')];
  const detail = document.querySelector('#host-detail');
  const selectHost = row => {
    rows.forEach(host => {
      const selected = host === row;
      host.classList.toggle('selected', selected);
      host.setAttribute('aria-pressed', String(selected));
    });
    detail.textContent = row ? `${row.dataset.user}@${row.dataset.address}:22` : '—';
  };
  rows.forEach(row => row.addEventListener('click', () => selectHost(row)));
  const filter = () => {
    const term = search.value.trim().toLocaleLowerCase();
    rows.forEach(row => { row.hidden = !`${row.dataset.name} ${row.dataset.tags}`.toLocaleLowerCase().includes(term); });
    const visible = rows.filter(row => !row.hidden);
    document.querySelector('#host-count').textContent = `${visible.length} 台主机`;
    document.querySelector('#no-hosts').hidden = visible.length > 0;
    if (!visible.some(row => row.classList.contains('selected'))) selectHost(visible[0]);
  };
  search.addEventListener('input', filter);
  document.querySelector('.terminal').addEventListener('keydown', event => {
    if (event.key === 'Escape') { search.value = ''; filter(); search.focus(); }
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      const visible = rows.filter(row => !row.hidden);
      if (!visible.length) return;
      event.preventDefault();
      const index = visible.findIndex(row => row.classList.contains('selected'));
      const direction = event.key === 'ArrowDown' ? 1 : -1;
      selectHost(visible[(index + direction + visible.length) % visible.length]);
    }
  });
  document.addEventListener('keydown', event => {
    if (event.key === '/' && !event.ctrlKey && !event.metaKey && !event.altKey && !event.target.closest('input, textarea, [contenteditable="true"]')) {
      event.preventDefault(); search.focus();
    }
  });
  document.querySelectorAll('[role="tablist"]').forEach(tablist => {
    const tabs = [...tablist.querySelectorAll('[role="tab"]')];
    const activate = tab => tabs.forEach(item => {
      const active = item === tab;
      item.setAttribute('aria-selected', String(active));
      item.tabIndex = active ? 0 : -1;
      document.getElementById(item.getAttribute('aria-controls')).hidden = !active;
    });
    tabs.forEach((tab, index) => {
      tab.addEventListener('click', () => activate(tab));
      tab.addEventListener('keydown', event => {
        let next;
        if (event.key === 'ArrowRight') next = (index + 1) % tabs.length;
        if (event.key === 'ArrowLeft') next = (index - 1 + tabs.length) % tabs.length;
        if (event.key === 'Home') next = 0;
        if (event.key === 'End') next = tabs.length - 1;
        if (next === undefined) return;
        event.preventDefault(); activate(tabs[next]); tabs[next].focus();
      });
    });
  });
  const status = document.querySelector('#copy-status');
  let toastTimer;
  const toast = message => {
    status.textContent = message;
    status.classList.add('visible');
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => status.classList.remove('visible'), 2600);
  };
  document.querySelectorAll('[data-copy], [data-copy-text]').forEach(button => {
    button.addEventListener('click', async () => {
      const value = button.dataset.copyText || document.getElementById(button.dataset.copy).textContent;
      try { await navigator.clipboard.writeText(value); toast('已复制到剪贴板'); }
      catch { toast('无法自动复制，请选中命令后手动复制。'); }
    });
  });
})();
