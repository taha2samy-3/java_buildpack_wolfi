const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;

const menu = document.querySelector('[data-menu]');
const nav = document.querySelector('[data-nav]');
if (menu && nav) {
  menu.addEventListener('click', () => {
    const open = nav.classList.toggle('open');
    menu.setAttribute('aria-expanded', String(open));
  });
}

const revealed = document.querySelectorAll('[data-reveal]');
if (reduced || !('IntersectionObserver' in window)) {
  revealed.forEach((el) => el.classList.add('visible'));
} else {
  document.documentElement.classList.add('motion');
  const io = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          entry.target.classList.add('visible');
          io.unobserve(entry.target);
        }
      }
    },
    { rootMargin: '0px 0px -8% 0px', threshold: 0.08 },
  );
  revealed.forEach((el) => io.observe(el));
}

const typing = document.querySelector('[data-typing]');
const outputs = [...document.querySelectorAll('[data-out]')];
if (typing && !reduced) {
  const text = typing.dataset.typing;
  typing.textContent = '';
  outputs.forEach((o) => o.classList.add('pending'));
  let i = 0;
  const type = () => {
    typing.textContent = text.slice(0, ++i);
    if (i < text.length) {
      setTimeout(type, 18 + Math.random() * 34);
    } else {
      outputs.forEach((o, n) => setTimeout(() => o.classList.remove('pending'), 380 + n * 260));
    }
  };
  setTimeout(type, 700);
}

document.querySelectorAll('.content pre, .release-body pre').forEach((pre) => {
  const code = pre.querySelector('code');
  if (!code || !navigator.clipboard) return;
  const button = document.createElement('button');
  button.className = 'copy';
  button.type = 'button';
  button.textContent = 'Copy';
  button.addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(code.innerText.replace(/^\$ /gm, ''));
      button.textContent = 'Copied';
      button.classList.add('done');
    } catch {
      button.textContent = 'Failed';
    }
    setTimeout(() => {
      button.textContent = 'Copy';
      button.classList.remove('done');
    }, 1600);
  });
  pre.appendChild(button);
});

const tocLinks = document.querySelectorAll('[data-toc]');
if (tocLinks.length && 'IntersectionObserver' in window) {
  const map = new Map([...tocLinks].map((a) => [a.dataset.toc, a]));
  const spy = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          tocLinks.forEach((a) => a.classList.remove('active'));
          map.get(entry.target.id)?.classList.add('active');
        }
      }
    },
    { rootMargin: '-20% 0px -70% 0px' },
  );
  map.forEach((_, id) => {
    const h = document.getElementById(id);
    if (h) spy.observe(h);
  });
}
