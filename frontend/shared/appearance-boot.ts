/** localStorage key holding both compiled modes; written by the appearance store. */
export const compiledAppearanceKey = 'ircord.appearance.compiled'

/**
 * Inline <head> script. Applies the last compiled appearance before the app bundle runs, so a
 * saved theme never flashes the default one. Must stay dependency-free and tiny.
 */
export const appearanceBootScript = `(function(){try{
var c=JSON.parse(localStorage.getItem('${compiledAppearanceKey}')||'null');if(!c)return;
var m=c.mode==='system'?(matchMedia('(prefers-color-scheme: light)').matches?'light':'dark'):c.mode;
var v=c[m];if(!v)return;var d=document.documentElement;
for(var k in v.attrs)d.setAttribute(k,v.attrs[k]);d.style.colorScheme=m;
var s=document.createElement('style');s.id='ircord-theme';s.textContent=v.css;document.head.appendChild(s);
}catch(e){}})()`
