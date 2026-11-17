import { compileAppearance, defaultAppearance } from './appearance'

/** localStorage key holding both compiled modes; written by the appearance store. */
export const compiledAppearanceKey = 'ircord.appearance.compiled'
/**
 * localStorage key for the signed-in account's kept personal theme: both compiled modes, the
 * filtered custom CSS and the theme itself. Written by the themes store, removed on sign-out and
 * whenever the app finds itself signed out, so it never reaches the sign-in screen or another
 * account.
 */
export const compiledThemeKey = 'ircord.theme.compiled'
/** sessionStorage key for the theme studio's unsaved draft, offered back after a reload. */
export const themeDraftKey = 'ircord.theme.draft'
/** `?safe` in the address loads the app without the personal theme. */
export const safeModePattern = /[?&]safe(?:[=&]|$)/

/**
 * Inline <head> script. Applies the last compiled appearance before the app bundle runs, so a
 * saved theme never flashes the default one. Must stay dependency-free and tiny.
 */
const documentAppearance = (mode: 'dark' | 'light') => {
  const { css, attrs } = compileAppearance(defaultAppearance, mode)
  return { css, attrs }
}
const defaults = {
  mode: 'system',
  dark: documentAppearance('dark'),
  light: documentAppearance('light'),
}

export const appearanceBootScript = `(function(){
var c,t;try{c=JSON.parse(localStorage.getItem('${compiledAppearanceKey}')||'null');t=JSON.parse(localStorage.getItem('${compiledThemeKey}')||'null')}catch(e){}
c=c||${JSON.stringify(defaults)};try{
if(${safeModePattern}.test(location.search))t=null;
var m=c.mode==='system'?(matchMedia('(prefers-color-scheme: light)').matches?'light':'dark'):c.mode;
var v=(t&&t[m])||c[m];if(!v)return;var d=document.documentElement;
for(var k in v.attrs)d.setAttribute(k,v.attrs[k]);d.style.colorScheme=m;
var s=document.createElement('style');s.id='ircord-theme';s.textContent=v.css;document.head.appendChild(s);
if(t&&t[m]&&t.userCss){s=document.createElement('style');s.id='ircord-user-css';s.textContent=t.userCss;document.head.appendChild(s)}
}catch(e){}})()`
