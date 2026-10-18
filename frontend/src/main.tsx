import { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './style.css';

const VIDEO = 'https://d8j0ntlcm91z4.cloudfront.net/user_38xzZboKViGWJOttwIXH07lWA1P/hf_20260418_063509_7d167302-4fd4-480b-8260-18ab572333d4.mp4';
const LOGO = 'M 128 192 L 128 256 L 64.5 256 L 32 223 L 0 192 L 0 128 L 64 128 Z M 256 192 L 256 256 L 192.5 256 L 160 223 L 128 192 L 128 128 L 192 128 Z M 128 64 L 128 128 L 64.5 128 L 32 95 L 0 64 L 0 0 L 64 0 Z M 256 64 L 256 128 L 192.5 128 L 160 95 L 128 64 L 128 0 L 192 0 Z';

function Arrow() { return <span aria-hidden="true">↗</span>; }

function App() {
  const [workspace, setWorkspace] = useState(location.hash === '#workspace');
  const [menu, setMenu] = useState(false);
  const [question, setQuestion] = useState('');
  const [playing, setPlaying] = useState(!matchMedia('(prefers-reduced-motion: reduce)').matches);
  const [videoFailed, setVideoFailed] = useState(false);
  const [info, setInfo] = useState<'graph' | 'about' | null>(null);
  const video = useRef<HTMLVideoElement>(null);
  const dialog = useRef<HTMLDialogElement>(null);
  const menuButton = useRef<HTMLButtonElement>(null);
  const drawer = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const changed = () => { setWorkspace(location.hash === '#workspace'); setMenu(false); };
    addEventListener('hashchange', changed);
    return () => removeEventListener('hashchange', changed);
  }, []);
  useEffect(() => {
    const view = document.getElementById('workspace-view')!;
    view.hidden = !workspace;
    document.body.classList.toggle('in-workspace', workspace);
    if (workspace) { window.scrollTo(0, 0); document.getElementById('question')?.focus(); }
  }, [workspace]);
  useEffect(() => {
    if (!video.current) return;
    if (playing && !workspace) video.current.play().catch(() => setPlaying(false));
    else video.current.pause();
  }, [playing, workspace]);
  useEffect(() => {
    const media = matchMedia('(prefers-reduced-motion: reduce)');
    const changed = () => { if (media.matches) setPlaying(false); };
    media.addEventListener('change', changed);
    return () => media.removeEventListener('change', changed);
  }, []);
  useEffect(() => {
    if (info) dialog.current?.showModal();
    else dialog.current?.close();
  }, [info]);
  useEffect(() => {
    if (!menu) return;
    drawer.current?.querySelector<HTMLButtonElement>('button')?.focus();
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { setMenu(false); menuButton.current?.focus(); }
      if (event.key === 'Tab') {
        const buttons = [...(drawer.current?.querySelectorAll<HTMLButtonElement>('button') ?? [])];
        const first = buttons[0], last = buttons[buttons.length - 1];
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
      }
    };
    addEventListener('keydown', key);
    return () => removeEventListener('keydown', key);
  }, [menu]);

  function openWorkspace(prompt?: string) {
    setMenu(false);
    location.hash = 'workspace';
    if (prompt?.trim()) {
      const input = document.getElementById('question') as HTMLTextAreaElement;
      input.value = prompt.trim();
      (document.getElementById('target') as HTMLSelectElement).value = '';
      (document.getElementById('asof') as HTMLInputElement).value = '';
      document.getElementById('ask-form')?.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
    }
  }
  function showInfo(value: 'graph' | 'about') { setMenu(false); setInfo(value); }

  return <>
    <section className="tron-hero relative h-screen w-full overflow-hidden bg-black" hidden={workspace} aria-label="TRON company brain">
      <div className="video-fallback" aria-hidden="true" />
      <video ref={video} className="hero-video absolute inset-0 h-full w-full object-cover" autoPlay={playing} loop muted playsInline preload="metadata" src={VIDEO} onError={() => { setVideoFailed(true); setPlaying(false); }} aria-hidden="true" />
      <div className="hero-shade absolute inset-0" aria-hidden="true" />
      <a href="#hero-question" className="skip-link" inert={menu}>skip to question</a>
      <nav className="hero-nav absolute inset-x-0 top-0 z-20 flex items-center justify-between gap-4 px-6 pt-6 md:px-10" aria-label="Main navigation" inert={menu}>
        <a href="#" className="brand-pill flex items-center gap-3 rounded-full bg-neutral-900/90 py-3 pl-4 pr-6 backdrop-blur" aria-label="TRON home"><svg viewBox="0 0 256 256" className="h-5 w-5" aria-hidden="true"><path d={LOGO} fill="#fff" /></svg><span>tron<span className="wordmark-dot">.</span></span></a>
        <div className="hidden items-center gap-1 rounded-full bg-neutral-900/90 px-3 py-2 backdrop-blur md:flex">
          <button className="nav-link" onClick={() => openWorkspace()}>workspace <Arrow /></button>
          <button className="nav-link" onClick={() => showInfo('graph')}>the graph</button>
          <button className="nav-link" onClick={() => showInfo('about')}>about tron</button>
          <a className="nav-link" href="https://github.com/Shashwat-911/tr0n-shashwat" target="_blank" rel="noreferrer">source <Arrow /></a>
        </div>
        <button className="hero-cta hidden rounded-full bg-white px-6 py-3 text-sm text-black transition-colors hover:bg-neutral-200 md:block" onClick={() => openWorkspace()}>explore workspace <Arrow /></button>
        <button ref={menuButton} className="menu-toggle rounded-full bg-neutral-900/90 backdrop-blur md:hidden" aria-label={menu ? 'Close navigation' : 'Open navigation'} aria-expanded={menu} aria-controls="mobile-navigation" onClick={() => setMenu(!menu)}><span aria-hidden="true">{menu ? '×' : '☰'}</span></button>
      </nav>
      {menu && <div className="mobile-navigation" id="mobile-navigation" ref={drawer} role="dialog" aria-modal="true" aria-label="Navigation">
        <button onClick={() => { setMenu(false); menuButton.current?.focus(); }}>close navigation ×</button>
        <button onClick={() => openWorkspace()}>workspace <Arrow /></button>
        <button onClick={() => showInfo('graph')}>the graph <Arrow /></button>
        <button onClick={() => showInfo('about')}>about tron <Arrow /></button>
        <button onClick={() => { setMenu(false); document.getElementById('hero-question')?.focus(); }}>ask a question <Arrow /></button>
        <p>trusted reasoning.<br />connected knowledge.</p>
      </div>}
      <div className="hero-content relative h-full w-full" inert={menu}>
        <div className="hero-kicker"><span className="signal-dot" /> company brain / graph hacks 2026</div>
        <div className="hero-index"><span className="diagonal" /><span>knowledge,<br />with a source.</span></div>
        <h1 className="hero-title"><span className="word-know">know</span><span className="word-your">your</span><span className="word-company">company<span className="title-period">.</span></span></h1>
        <p className="hero-description">the people. the decisions.<br />the reasons behind them.<br /><span>one connected source of context.</span></p>
        <div className="hero-bottom">
          <div className="hero-question-block">
            <label htmlFor="hero-question">a better answer starts with a connection.</label>
            <form className="hero-question-form" onSubmit={event => { event.preventDefault(); openWorkspace(question || 'Who knows the Payments service best?'); }}>
              <input id="hero-question" value={question} onChange={event => setQuestion(event.target.value)} maxLength={2000} placeholder="who knows payments best?" aria-label="Ask TRON a company question" />
              <button type="submit" aria-label="Ask TRON">ask tron <Arrow /></button>
            </form>
            <span className="hero-footnote">local-first · source-grounded · powered by falkordb</span>
          </div>
          <div className="hero-cards">
            <button className="glass-card workflow-card" onClick={() => openWorkspace()}><div className="card-heading"><span className="pixel-number">05</span><Arrow /></div><strong>ways to find your answer.</strong><p>ownership, expertise, decisions,<br />incident history & handoffs.</p></button>
            <button className="glass-card evidence-card" onClick={() => showInfo('graph')}><div className="card-heading"><span className="connection-icon" aria-hidden="true">○—○—○</span><span>01 / context</span></div><strong>follow the evidence.</strong><p>every supported answer traces<br />back to your company graph.</p></button>
          </div>
        </div>
      </div>
      <button className="motion-control" inert={menu} onClick={() => setPlaying(!playing)} disabled={videoFailed} aria-label={playing ? 'Pause background video' : 'Play background video'}>{videoFailed ? 'still background' : playing ? 'Ⅱ pause motion' : '▷ play motion'}</button>
      <div className="hero-edition" aria-hidden="true">trusted reasoning & organizational nexus</div>
    </section>
    <dialog ref={dialog} className="info-dialog" onCancel={() => setInfo(null)} onClick={event => { if (event.target === dialog.current) setInfo(null); }}>
      <button className="dialog-close" onClick={() => setInfo(null)} aria-label="Close details">×</button>
      <p className="dialog-kicker">tron / {info === 'graph' ? 'connected evidence' : 'company brain'}</p>
      <h2>{info === 'graph' ? 'the answer is in the connections.' : 'context for the people doing the work.'}</h2>
      {info === 'graph' ? <><p>FalkorDB connects people, services, repositories, incidents, documents, and decisions. TRON follows those relationships and brings the supporting sources into your answer.</p><div className="graph-example">person → pull request → repository ← service</div><p>Inspect a source, follow its neighbors, or ask who owned a service at an earlier date. Facts, computed inferences, and recommendations stay visibly distinct.</p></> : <><p>TRON is a local Company Brain prototype for Graph Hacks, Track 03. Five deterministic workflows work without an API key. Optional local Ollama adds bounded, evidence-grounded questions.</p><p>The demo uses synthetic company data. Drafts stay local; TRON does not send messages or execute external actions. Source selection and citations are inspectable.</p></>}
      <button className="dialog-action" onClick={() => { setInfo(null); openWorkspace(); }}>open the workspace <Arrow /></button>
    </dialog>
  </>;
}

createRoot(document.getElementById('hero-root')!).render(<App />);
