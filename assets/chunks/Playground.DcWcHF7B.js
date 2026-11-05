import{d as cd,q as dd,v as ud,g as fd,$ as pd,o as ye,c as be,j as Q,F as yn,B as bn,t as Ne,k as wn,e as vn,ah as Rs,ai as md,Z as gd,n as kn,a0 as yd,a as Ro,aj as Eo,p as Ze,G as Lo,ak as bd,h as Es,P as wd}from"./framework.k9nOhppk.js";let rr=[],pl=[];(()=>{let i="lc,34,7n,7,7b,19,,,,2,,2,,,20,b,1c,l,g,,2t,7,2,6,2,2,,4,z,,u,r,2j,b,1m,9,9,,o,4,,9,,3,,5,17,3,1n,9,16,o,,x,1i,3,,i,,7,a,2,t,3,1k,,,7,2,2,2,3,9,,a,2,q,,2,3,1k,,,5,4,2,2,3,3,,u,2,3,,b,3,1k,,,8,,3,,3,k,2,m,6,,3,1k,,,7,2,2,2,3,7,3,a,2,u,,1n,5,3,3,,4,9,,14,5,1j,,,7,,3,,4,7,2,b,2,t,3,1k,,,7,,3,,4,7,2,b,2,f,,c,4,1j,2,,7,,3,,4,9,,a,2,t,3,1y,,4,6,,,,8,i,2,1p,,,8,c,8,2q,,,a,b,7,21,2,r,,,,,,4,2,1d,k,,2,5,b,,10,9,,2u,b,,6,n,4,4,3,g,4,d,,,3,6,,f,,jj,3,qa,4,s,3,t,2,u,2,1s,w,9,,19,3,,,39,2,y,,3a,c,4,c,63,5,1l,a,,,,,2,o,2,,1c,1a,2,c,k,5,1b,h,12,9,c,3,u,d,1k,e,1c,k,48,3,,l,4,,6,,2,3,5i,1s,ek,,5f,x,2da,3,3x,,2o,w,fe,6,2x,2,n9w,4,,a,w,2,28,2,7k,,3,,4,,n,5,4,,2b,2,1e,i,q,i,d,,12,8,p,d,18,4,1b,e,10,,1v,e,c,,8,2,1a,,1f,,,3,2,2,5,2,,,15,5,5,2,6k,8,,2,fn4,,kh,g,g,g,a6,2,gt,,6a,,45,5,1ae,3,,2,5,4,14,3,4,,4l,2,fx,4,1t,5,8t,2,25,6,1y,b,1d,4,3e,3,1h,f,15,,2,2,a,4,19,b,7,,1p,3,10,e,g,2,18,,c,3,1c,e,8,4,,2,2k,c,6,,2,,4d,c,l,4,1j,2,,7,2,2,2,3,9,,a,2,2,7,3,5,1v,9,,,2,,,4,,5,,,e,2,2a,i,n,,29,k,6j,7,2,9,r,2,2a,h,2y,d,2t,3,2,a,74,f,6t,6,,2,2,4,,,,2,3x,7,2,7,3,,s,a,14,7,,4,8,,9,b,1a,g,5i,8,5j,8,,8,2a,m,,e,3e,6,3,,,2,,7,,,1u,5,,2,,5,9n,4,9,2,,,1c,7,3,5,n,,44l,,6,f,8ug,i,1xc,5,1n,7,t4,,,1j,7,4,29,,b,2,f57,2,3mp,1a,2,n,f2,5,3,6,8,8,2,7,u,4,44,3,1iz,1j,4,1e,8,,e,,m,5,,f,11s,7,,h,2,7,,2,,5,2s,,4g,7,af,,1p,4,e4,4,72,2,6r,,2,,7,2,5,,d6,7,31,7,240,5".split(",").map(e=>e?parseInt(e,36):1);for(let e=0,t=0;e<i.length;e++)(e%2?pl:rr).push(t=t+i[e])})();function vd(i){if(i<768)return!1;for(let e=0,t=rr.length;;){let n=e+t>>1;if(i<rr[n])t=n;else if(i>=pl[n])e=n+1;else return!0;if(e==t)return!1}}function Bo(i){return i>=127462&&i<=127487}const _o=8205;function kd(i,e,t=!0,n=!0){return(t?ml:xd)(i,e,n)}function ml(i,e,t){if(e==i.length)return e;e&&gl(i.charCodeAt(e))&&yl(i.charCodeAt(e-1))&&e--;let n=Ls(i,e);for(e+=Po(n);e<i.length;){let s=Ls(i,e);if(n==_o||s==_o||t&&vd(s))e+=Po(s),n=s;else if(Bo(s)){let r=0,o=e-2;for(;o>=0&&Bo(Ls(i,o));)r++,o-=2;if(r%2==0)break;e+=2}else break}return e}function xd(i,e,t){for(;e>1;){let n=ml(i,e-2,t);if(n<e)return n;e--}return 0}function Ls(i,e){let t=i.charCodeAt(e);if(!yl(t)||e+1==i.length)return t;let n=i.charCodeAt(e+1);return gl(n)?(t-55296<<10)+(n-56320)+65536:t}function gl(i){return i>=56320&&i<57344}function yl(i){return i>=55296&&i<56320}function Po(i){return i<65536?1:2}class j{lineAt(e){if(e<0||e>this.length)throw new RangeError(`Invalid position ${e} in document of length ${this.length}`);return this.lineInner(e,!1,1,0)}line(e){if(e<1||e>this.lines)throw new RangeError(`Invalid line number ${e} in ${this.lines}-line document`);return this.lineInner(e,!0,1,0)}replace(e,t,n){[e,t]=rn(this,e,t);let s=[];return this.decompose(0,e,s,2),n.length&&n.decompose(0,n.length,s,3),this.decompose(t,this.length,s,1),st.from(s,this.length-(t-e)+n.length)}append(e){return this.replace(this.length,this.length,e)}slice(e,t=this.length){[e,t]=rn(this,e,t);let n=[];return this.decompose(e,t,n,0),st.from(n,t-e)}eq(e){if(e==this)return!0;if(e.length!=this.length||e.lines!=this.lines)return!1;let t=this.scanIdentical(e,1),n=this.length-this.scanIdentical(e,-1),s=new Dn(this),r=new Dn(e);for(let o=t,a=t;;){if(s.next(o),r.next(o),o=0,s.lineBreak!=r.lineBreak||s.done!=r.done||s.value!=r.value)return!1;if(a+=s.value.length,s.done||a>=n)return!0}}iter(e=1){return new Dn(this,e)}iterRange(e,t=this.length){return new bl(this,e,t)}iterLines(e,t){let n;if(e==null)n=this.iter();else{t==null&&(t=this.lines+1);let s=this.line(e).from;n=this.iterRange(s,Math.max(s,t==this.lines+1?this.length:t<=1?0:this.line(t-1).to))}return new wl(n)}toString(){return this.sliceString(0)}toJSON(){let e=[];return this.flatten(e),e}constructor(){}static of(e){if(e.length==0)throw new RangeError("A document must have at least one line");return e.length==1&&!e[0]?j.empty:e.length<=32?new ne(e):st.from(ne.split(e,[]))}}class ne extends j{constructor(e,t=Sd(e)){super(),this.text=e,this.length=t}get lines(){return this.text.length}get children(){return null}lineInner(e,t,n,s){for(let r=0;;r++){let o=this.text[r],a=s+o.length;if((t?n:a)>=e)return new Cd(s,a,n,o);s=a+1,n++}}decompose(e,t,n,s){let r=e<=0&&t>=this.length?this:new ne(No(this.text,e,t),Math.min(t,this.length)-Math.max(0,e));if(s&1){let o=n.pop(),a=Mi(r.text,o.text.slice(),0,r.length);if(a.length<=32)n.push(new ne(a,o.length+r.length));else{let l=a.length>>1;n.push(new ne(a.slice(0,l)),new ne(a.slice(l)))}}else n.push(r)}replace(e,t,n){if(!(n instanceof ne))return super.replace(e,t,n);[e,t]=rn(this,e,t);let s=Mi(this.text,Mi(n.text,No(this.text,0,e)),t),r=this.length+n.length-(t-e);return s.length<=32?new ne(s,r):st.from(ne.split(s,[]),r)}sliceString(e,t=this.length,n=`
`){[e,t]=rn(this,e,t);let s="";for(let r=0,o=0;r<=t&&o<this.text.length;o++){let a=this.text[o],l=r+a.length;r>e&&o&&(s+=n),e<l&&t>r&&(s+=a.slice(Math.max(0,e-r),t-r)),r=l+1}return s}flatten(e){for(let t of this.text)e.push(t)}scanIdentical(){return 0}static split(e,t){let n=[],s=-1;for(let r of e)n.push(r),s+=r.length+1,n.length==32&&(t.push(new ne(n,s)),n=[],s=-1);return s>-1&&t.push(new ne(n,s)),t}}class st extends j{constructor(e,t){super(),this.children=e,this.length=t,this.lines=0;for(let n of e)this.lines+=n.lines}lineInner(e,t,n,s){for(let r=0;;r++){let o=this.children[r],a=s+o.length,l=n+o.lines-1;if((t?l:a)>=e)return o.lineInner(e,t,n,s);s=a+1,n=l+1}}decompose(e,t,n,s){for(let r=0,o=0;o<=t&&r<this.children.length;r++){let a=this.children[r],l=o+a.length;if(e<=l&&t>=o){let h=s&((o<=e?1:0)|(l>=t?2:0));o>=e&&l<=t&&!h?n.push(a):a.decompose(e-o,t-o,n,h)}o=l+1}}replace(e,t,n){if([e,t]=rn(this,e,t),n.lines<this.lines)for(let s=0,r=0;s<this.children.length;s++){let o=this.children[s],a=r+o.length;if(e>=r&&t<=a){let l=o.replace(e-r,t-r,n),h=this.lines-o.lines+l.lines;if(l.lines<h>>4&&l.lines>h>>6){let c=this.children.slice();return c[s]=l,new st(c,this.length-(t-e)+n.length)}return super.replace(r,a,l)}r=a+1}return super.replace(e,t,n)}sliceString(e,t=this.length,n=`
`){[e,t]=rn(this,e,t);let s="";for(let r=0,o=0;r<this.children.length&&o<=t;r++){let a=this.children[r],l=o+a.length;o>e&&r&&(s+=n),e<l&&t>o&&(s+=a.sliceString(e-o,t-o,n)),o=l+1}return s}flatten(e){for(let t of this.children)t.flatten(e)}scanIdentical(e,t){if(!(e instanceof st))return 0;let n=0,[s,r,o,a]=t>0?[0,0,this.children.length,e.children.length]:[this.children.length-1,e.children.length-1,-1,-1];for(;;s+=t,r+=t){if(s==o||r==a)return n;let l=this.children[s],h=e.children[r];if(l!=h)return n+l.scanIdentical(h,t);n+=l.length+1}}static from(e,t=e.reduce((n,s)=>n+s.length+1,-1)){let n=0;for(let f of e)n+=f.lines;if(n<32){let f=[];for(let p of e)p.flatten(f);return new ne(f,t)}let s=Math.max(32,n>>5),r=s<<1,o=s>>1,a=[],l=0,h=-1,c=[];function d(f){let p;if(f.lines>r&&f instanceof st)for(let m of f.children)d(m);else f.lines>o&&(l>o||!l)?(u(),a.push(f)):f instanceof ne&&l&&(p=c[c.length-1])instanceof ne&&f.lines+p.lines<=32?(l+=f.lines,h+=f.length+1,c[c.length-1]=new ne(p.text.concat(f.text),p.length+1+f.length)):(l+f.lines>s&&u(),l+=f.lines,h+=f.length+1,c.push(f))}function u(){l!=0&&(a.push(c.length==1?c[0]:st.from(c,h)),h=-1,l=c.length=0)}for(let f of e)d(f);return u(),a.length==1?a[0]:new st(a,t)}}j.empty=new ne([""],0);function Sd(i){let e=-1;for(let t of i)e+=t.length+1;return e}function Mi(i,e,t=0,n=1e9){for(let s=0,r=0,o=!0;r<i.length&&s<=n;r++){let a=i[r],l=s+a.length;l>=t&&(l>n&&(a=a.slice(0,n-s)),s<t&&(a=a.slice(t-s)),o?(e[e.length-1]+=a,o=!1):e.push(a)),s=l+1}return e}function No(i,e,t){return Mi(i,[""],e,t)}class Dn{constructor(e,t=1){this.dir=t,this.done=!1,this.lineBreak=!1,this.value="",this.nodes=[e],this.offsets=[t>0?1:(e instanceof ne?e.text.length:e.children.length)<<1]}nextInner(e,t){for(this.done=this.lineBreak=!1;;){let n=this.nodes.length-1,s=this.nodes[n],r=this.offsets[n],o=r>>1,a=s instanceof ne?s.text.length:s.children.length;if(o==(t>0?a:0)){if(n==0)return this.done=!0,this.value="",this;t>0&&this.offsets[n-1]++,this.nodes.pop(),this.offsets.pop()}else if((r&1)==(t>0?0:1)){if(this.offsets[n]+=t,e==0)return this.lineBreak=!0,this.value=`
`,this;e--}else if(s instanceof ne){let l=s.text[o+(t<0?-1:0)];if(this.offsets[n]+=t,l.length>Math.max(0,e))return this.value=e==0?l:t>0?l.slice(e):l.slice(0,l.length-e),this;e-=l.length}else{let l=s.children[o+(t<0?-1:0)];e>l.length?(e-=l.length,this.offsets[n]+=t):(t<0&&this.offsets[n]--,this.nodes.push(l),this.offsets.push(t>0?1:(l instanceof ne?l.text.length:l.children.length)<<1))}}}next(e=0){return e<0&&(this.nextInner(-e,-this.dir),e=this.value.length),this.nextInner(e,this.dir)}}class bl{constructor(e,t,n){this.value="",this.done=!1,this.cursor=new Dn(e,t>n?-1:1),this.pos=t>n?e.length:0,this.from=Math.min(t,n),this.to=Math.max(t,n)}nextInner(e,t){if(t<0?this.pos<=this.from:this.pos>=this.to)return this.value="",this.done=!0,this;e+=Math.max(0,t<0?this.pos-this.to:this.from-this.pos);let n=t<0?this.pos-this.from:this.to-this.pos;e>n&&(e=n),n-=e;let{value:s}=this.cursor.next(e);return this.pos+=(s.length+e)*t,this.value=s.length<=n?s:t<0?s.slice(s.length-n):s.slice(0,n),this.done=!this.value,this}next(e=0){return e<0?e=Math.max(e,this.from-this.pos):e>0&&(e=Math.min(e,this.to-this.pos)),this.nextInner(e,this.cursor.dir)}get lineBreak(){return this.cursor.lineBreak&&this.value!=""}}class wl{constructor(e){this.inner=e,this.afterBreak=!0,this.value="",this.done=!1}next(e=0){let{done:t,lineBreak:n,value:s}=this.inner.next(e);return t&&this.afterBreak?(this.value="",this.afterBreak=!1):t?(this.done=!0,this.value=""):n?this.afterBreak?this.value="":(this.afterBreak=!0,this.next()):(this.value=s,this.afterBreak=!1),this}get lineBreak(){return!1}}typeof Symbol<"u"&&(j.prototype[Symbol.iterator]=function(){return this.iter()},Dn.prototype[Symbol.iterator]=bl.prototype[Symbol.iterator]=wl.prototype[Symbol.iterator]=function(){return this});class Cd{constructor(e,t,n,s){this.from=e,this.to=t,this.number=n,this.text=s}get length(){return this.to-this.from}}function rn(i,e,t){return e=Math.max(0,Math.min(i.length,e)),[e,Math.max(e,Math.min(i.length,t))]}function he(i,e,t=!0,n=!0){return kd(i,e,t,n)}function Ad(i){return i>=56320&&i<57344}function Td(i){return i>=55296&&i<56320}function _e(i,e){let t=i.charCodeAt(e);if(!Td(t)||e+1==i.length)return t;let n=i.charCodeAt(e+1);return Ad(n)?(t-55296<<10)+(n-56320)+65536:t}function Gr(i){return i<=65535?String.fromCharCode(i):(i-=65536,String.fromCharCode((i>>10)+55296,(i&1023)+56320))}function rt(i){return i<65536?1:2}const or=/\r\n?|\n/;var Se=function(i){return i[i.Simple=0]="Simple",i[i.TrackDel=1]="TrackDel",i[i.TrackBefore=2]="TrackBefore",i[i.TrackAfter=3]="TrackAfter",i}(Se||(Se={}));class dt{constructor(e){this.sections=e}get length(){let e=0;for(let t=0;t<this.sections.length;t+=2)e+=this.sections[t];return e}get newLength(){let e=0;for(let t=0;t<this.sections.length;t+=2){let n=this.sections[t+1];e+=n<0?this.sections[t]:n}return e}get empty(){return this.sections.length==0||this.sections.length==2&&this.sections[1]<0}iterGaps(e){for(let t=0,n=0,s=0;t<this.sections.length;){let r=this.sections[t++],o=this.sections[t++];o<0?(e(n,s,r),s+=r):s+=o,n+=r}}iterChangedRanges(e,t=!1){ar(this,e,t)}get invertedDesc(){let e=[];for(let t=0;t<this.sections.length;){let n=this.sections[t++],s=this.sections[t++];s<0?e.push(n,s):e.push(s,n)}return new dt(e)}composeDesc(e){return this.empty?e:e.empty?this:vl(this,e)}mapDesc(e,t=!1){return e.empty?this:lr(this,e,t)}mapPos(e,t=-1,n=Se.Simple){let s=0,r=0;for(let o=0;o<this.sections.length;){let a=this.sections[o++],l=this.sections[o++],h=s+a;if(l<0){if(h>e)return r+(e-s);r+=a}else{if(n!=Se.Simple&&h>=e&&(n==Se.TrackDel&&s<e&&h>e||n==Se.TrackBefore&&s<e||n==Se.TrackAfter&&h>e))return null;if(h>e||h==e&&t<0&&!a)return e==s||t<0?r:r+l;r+=l}s=h}if(e>s)throw new RangeError(`Position ${e} is out of range for changeset of length ${s}`);return r}touchesRange(e,t=e){for(let n=0,s=0;n<this.sections.length&&s<=t;){let r=this.sections[n++],o=this.sections[n++],a=s+r;if(o>=0&&s<=t&&a>=e)return s<e&&a>t?"cover":!0;s=a}return!1}toString(){let e="";for(let t=0;t<this.sections.length;){let n=this.sections[t++],s=this.sections[t++];e+=(e?" ":"")+n+(s>=0?":"+s:"")}return e}toJSON(){return this.sections}static fromJSON(e){if(!Array.isArray(e)||e.length%2||e.some(t=>typeof t!="number"))throw new RangeError("Invalid JSON representation of ChangeDesc");return new dt(e)}static create(e){return new dt(e)}}class oe extends dt{constructor(e,t){super(e),this.inserted=t}apply(e){if(this.length!=e.length)throw new RangeError("Applying change set to a document with the wrong length");return ar(this,(t,n,s,r,o)=>e=e.replace(s,s+(n-t),o),!1),e}mapDesc(e,t=!1){return lr(this,e,t,!0)}invert(e){let t=this.sections.slice(),n=[];for(let s=0,r=0;s<t.length;s+=2){let o=t[s],a=t[s+1];if(a>=0){t[s]=a,t[s+1]=o;let l=s>>1;for(;n.length<l;)n.push(j.empty);n.push(o?e.slice(r,r+o):j.empty)}r+=o}return new oe(t,n)}compose(e){return this.empty?e:e.empty?this:vl(this,e,!0)}map(e,t=!1){return e.empty?this:lr(this,e,t,!0)}iterChanges(e,t=!1){ar(this,e,t)}get desc(){return dt.create(this.sections)}filter(e){let t=[],n=[],s=[],r=new qn(this);e:for(let o=0,a=0;;){let l=o==e.length?1e9:e[o++];for(;a<l||a==l&&r.len==0;){if(r.done)break e;let c=Math.min(r.len,l-a);we(s,c,-1);let d=r.ins==-1?-1:r.off==0?r.ins:0;we(t,c,d),d>0&&St(n,t,r.text),r.forward(c),a+=c}let h=e[o++];for(;a<h;){if(r.done)break e;let c=Math.min(r.len,h-a);we(t,c,-1),we(s,c,r.ins==-1?-1:r.off==0?r.ins:0),r.forward(c),a+=c}}return{changes:new oe(t,n),filtered:dt.create(s)}}toJSON(){let e=[];for(let t=0;t<this.sections.length;t+=2){let n=this.sections[t],s=this.sections[t+1];s<0?e.push(n):s==0?e.push([n]):e.push([n].concat(this.inserted[t>>1].toJSON()))}return e}static of(e,t,n){let s=[],r=[],o=0,a=null;function l(c=!1){if(!c&&!s.length)return;o<t&&we(s,t-o,-1);let d=new oe(s,r);a=a?a.compose(d.map(a)):d,s=[],r=[],o=0}function h(c){if(Array.isArray(c))for(let d of c)h(d);else if(c instanceof oe){if(c.length!=t)throw new RangeError(`Mismatched change set length (got ${c.length}, expected ${t})`);l(),a=a?a.compose(c.map(a)):c}else{let{from:d,to:u=d,insert:f}=c;if(d>u||d<0||u>t)throw new RangeError(`Invalid change range ${d} to ${u} (in doc of length ${t})`);let p=f?typeof f=="string"?j.of(f.split(n||or)):f:j.empty,m=p.length;if(d==u&&m==0)return;d<o&&l(),d>o&&we(s,d-o,-1),we(s,u-d,m),St(r,s,p),o=u}}return h(e),l(!a),a}static empty(e){return new oe(e?[e,-1]:[],[])}static fromJSON(e){if(!Array.isArray(e))throw new RangeError("Invalid JSON representation of ChangeSet");let t=[],n=[];for(let s=0;s<e.length;s++){let r=e[s];if(typeof r=="number")t.push(r,-1);else{if(!Array.isArray(r)||typeof r[0]!="number"||r.some((o,a)=>a&&typeof o!="string"))throw new RangeError("Invalid JSON representation of ChangeSet");if(r.length==1)t.push(r[0],0);else{for(;n.length<s;)n.push(j.empty);n[s]=j.of(r.slice(1)),t.push(r[0],n[s].length)}}}return new oe(t,n)}static createSet(e,t){return new oe(e,t)}}function we(i,e,t,n=!1){if(e==0&&t<=0)return;let s=i.length-2;s>=0&&t<=0&&t==i[s+1]?i[s]+=e:s>=0&&e==0&&i[s]==0?i[s+1]+=t:n?(i[s]+=e,i[s+1]+=t):i.push(e,t)}function St(i,e,t){if(t.length==0)return;let n=e.length-2>>1;if(n<i.length)i[i.length-1]=i[i.length-1].append(t);else{for(;i.length<n;)i.push(j.empty);i.push(t)}}function ar(i,e,t){let n=i.inserted;for(let s=0,r=0,o=0;o<i.sections.length;){let a=i.sections[o++],l=i.sections[o++];if(l<0)s+=a,r+=a;else{let h=s,c=r,d=j.empty;for(;h+=a,c+=l,l&&n&&(d=d.append(n[o-2>>1])),!(t||o==i.sections.length||i.sections[o+1]<0);)a=i.sections[o++],l=i.sections[o++];e(s,h,r,c,d),s=h,r=c}}}function lr(i,e,t,n=!1){let s=[],r=n?[]:null,o=new qn(i),a=new qn(e);for(let l=-1;;){if(o.done&&a.len||a.done&&o.len)throw new Error("Mismatched change set lengths");if(o.ins==-1&&a.ins==-1){let h=Math.min(o.len,a.len);we(s,h,-1),o.forward(h),a.forward(h)}else if(a.ins>=0&&(o.ins<0||l==o.i||o.off==0&&(a.len<o.len||a.len==o.len&&!t))){let h=a.len;for(we(s,a.ins,-1);h;){let c=Math.min(o.len,h);o.ins>=0&&l<o.i&&o.len<=c&&(we(s,0,o.ins),r&&St(r,s,o.text),l=o.i),o.forward(c),h-=c}a.next()}else if(o.ins>=0){let h=0,c=o.len;for(;c;)if(a.ins==-1){let d=Math.min(c,a.len);h+=d,c-=d,a.forward(d)}else if(a.ins==0&&a.len<c)c-=a.len,a.next();else break;we(s,h,l<o.i?o.ins:0),r&&l<o.i&&St(r,s,o.text),l=o.i,o.forward(o.len-c)}else{if(o.done&&a.done)return r?oe.createSet(s,r):dt.create(s);throw new Error("Mismatched change set lengths")}}}function vl(i,e,t=!1){let n=[],s=t?[]:null,r=new qn(i),o=new qn(e);for(let a=!1;;){if(r.done&&o.done)return s?oe.createSet(n,s):dt.create(n);if(r.ins==0)we(n,r.len,0,a),r.next();else if(o.len==0&&!o.done)we(n,0,o.ins,a),s&&St(s,n,o.text),o.next();else{if(r.done||o.done)throw new Error("Mismatched change set lengths");{let l=Math.min(r.len2,o.len),h=n.length;if(r.ins==-1){let c=o.ins==-1?-1:o.off?0:o.ins;we(n,l,c,a),s&&c&&St(s,n,o.text)}else o.ins==-1?(we(n,r.off?0:r.len,l,a),s&&St(s,n,r.textBit(l))):(we(n,r.off?0:r.len,o.off?0:o.ins,a),s&&!o.off&&St(s,n,o.text));a=(r.ins>l||o.ins>=0&&o.len>l)&&(a||n.length>h),r.forward2(l),o.forward(l)}}}}class qn{constructor(e){this.set=e,this.i=0,this.next()}next(){let{sections:e}=this.set;this.i<e.length?(this.len=e[this.i++],this.ins=e[this.i++]):(this.len=0,this.ins=-2),this.off=0}get done(){return this.ins==-2}get len2(){return this.ins<0?this.len:this.ins}get text(){let{inserted:e}=this.set,t=this.i-2>>1;return t>=e.length?j.empty:e[t]}textBit(e){let{inserted:t}=this.set,n=this.i-2>>1;return n>=t.length&&!e?j.empty:t[n].slice(this.off,e==null?void 0:this.off+e)}forward(e){e==this.len?this.next():(this.len-=e,this.off+=e)}forward2(e){this.ins==-1?this.forward(e):e==this.ins?this.next():(this.ins-=e,this.off+=e)}}class kt{constructor(e,t,n,s){this.from=e,this.to=t,this.flags=n,this.goalColumn=s}get anchor(){return this.flags&32?this.to:this.from}get head(){return this.flags&32?this.from:this.to}get empty(){return this.from==this.to}get assoc(){return this.flags&8?-1:this.flags&16?1:0}get undirectional(){return(this.flags&64)>0}get bidiLevel(){let e=this.flags&7;return e==7?null:e}map(e,t=-1){let n,s;return this.empty?n=s=e.mapPos(this.from,t):(n=e.mapPos(this.from,1),s=e.mapPos(this.to,-1)),n==this.from&&s==this.to?this:new kt(n,s,this.flags,this.goalColumn)}extend(e,t=e,n=0){if(e<=this.anchor&&t>=this.anchor)return b.range(e,t,void 0,void 0,n);let s=Math.abs(e-this.anchor)>Math.abs(t-this.anchor)?e:t;return b.range(this.anchor,s,void 0,void 0,n)}eq(e,t=!1){return this.anchor==e.anchor&&this.head==e.head&&this.goalColumn==e.goalColumn&&(!t||!this.empty||this.assoc==e.assoc)}toJSON(){return{anchor:this.anchor,head:this.head}}static fromJSON(e){if(!e||typeof e.anchor!="number"||typeof e.head!="number")throw new RangeError("Invalid JSON representation for SelectionRange");return b.range(e.anchor,e.head)}static create(e,t,n,s){return new kt(e,t,n,s)}}class b{constructor(e,t){this.ranges=e,this.mainIndex=t}map(e,t=-1){return e.empty?this:b.create(this.ranges.map(n=>n.map(e,t)),this.mainIndex)}eq(e,t=!1){if(this.ranges.length!=e.ranges.length||this.mainIndex!=e.mainIndex)return!1;for(let n=0;n<this.ranges.length;n++)if(!this.ranges[n].eq(e.ranges[n],t))return!1;return!0}get main(){return this.ranges[this.mainIndex]}asSingle(){return this.ranges.length==1?this:new b([this.main],0)}addRange(e,t=!0){return b.create([e].concat(this.ranges),t?0:this.mainIndex+1)}replaceRange(e,t=this.mainIndex){let n=this.ranges.slice();return n[t]=e,b.create(n,this.mainIndex)}toJSON(){return{ranges:this.ranges.map(e=>e.toJSON()),main:this.mainIndex}}static fromJSON(e){if(!e||!Array.isArray(e.ranges)||typeof e.main!="number"||e.main>=e.ranges.length)throw new RangeError("Invalid JSON representation for EditorSelection");return new b(e.ranges.map(t=>kt.fromJSON(t)),e.main)}static single(e,t=e){return new b([b.range(e,t)],0)}static create(e,t=0){if(e.length==0)throw new RangeError("A selection needs at least one range");for(let n=0,s=0;s<e.length;s++){let r=e[s];if(r.empty?r.from<=n:r.from<n)return b.normalized(e.slice(),t);n=r.to}return new b(e,t)}static cursor(e,t=0,n,s){return kt.create(e,e,(t==0?0:t<0?8:16)|(n==null?7:Math.min(6,n)),s)}static range(e,t,n,s,r){let o=s==null?7:Math.min(6,s);return!r&&e!=t&&(r=t<e?1:-1),r&&(o|=r<0?8:16),t<e?kt.create(t,e,o|32,n):kt.create(e,t,o,n)}static undirectionalRange(e,t){return kt.create(e,t,64,void 0)}static normalized(e,t=0){let n=e[t];e.sort((s,r)=>s.from-r.from),t=e.indexOf(n);for(let s=1;s<e.length;s++){let r=e[s],o=e[s-1];if(r.empty?r.from<=o.to:r.from<o.to){let a=o.from,l=Math.max(r.to,o.to);s<=t&&t--,e.splice(--s,2,r.anchor>r.head?b.range(l,a):b.range(a,l))}}return new b(e,t)}}function kl(i,e){for(let t of i.ranges)if(t.to>e)throw new RangeError("Selection points outside of document")}let Yr=0;class D{constructor(e,t,n,s,r){this.combine=e,this.compareInput=t,this.compare=n,this.isStatic=s,this.id=Yr++,this.default=e([]),this.extensions=typeof r=="function"?r(this):r}get reader(){return this}static define(e={}){return new D(e.combine||(t=>t),e.compareInput||((t,n)=>t===n),e.compare||(e.combine?(t,n)=>t===n:Jr),!!e.static,e.enables)}of(e){return new Oi([],this,0,e)}compute(e,t){if(this.isStatic)throw new Error("Can't compute a static facet");return new Oi(e,this,1,t)}computeN(e,t){if(this.isStatic)throw new Error("Can't compute a static facet");return new Oi(e,this,2,t)}from(e,t){return t||(t=n=>n),this.compute([e],n=>t(n.field(e)))}}function Jr(i,e){return i==e||i.length==e.length&&i.every((t,n)=>t===e[n])}class Oi{constructor(e,t,n,s){this.dependencies=e,this.facet=t,this.type=n,this.value=s,this.id=Yr++}dynamicSlot(e){var t;let n=this.value,s=this.facet.compareInput,r=this.id,o=e[r]>>1,a=this.type==2,l=!1,h=!1,c=[];for(let d of this.dependencies)d=="doc"?l=!0:d=="selection"?h=!0:((t=e[d.id])!==null&&t!==void 0?t:1)&1||c.push(e[d.id]);return{create(d){return d.values[o]=n(d),1},update(d,u){if(l&&u.docChanged||h&&(u.docChanged||u.selection)||hr(d,c)){let f=n(d);if(a?!Wo(f,d.values[o],s):!s(f,d.values[o]))return d.values[o]=f,1}return 0},reconfigure:(d,u)=>{let f,p=u.config.address[r];if(p!=null){let m=Ni(u,p);if(this.dependencies.every(g=>g instanceof D?u.facet(g)===d.facet(g):g instanceof me?u.field(g,!1)==d.field(g,!1):!0)||(a?Wo(f=n(d),m,s):s(f=n(d),m)))return d.values[o]=m,0}else f=n(d);return d.values[o]=f,1}}}get extension(){return this}}function Wo(i,e,t){if(i.length!=e.length)return!1;for(let n=0;n<i.length;n++)if(!t(i[n],e[n]))return!1;return!0}function hr(i,e){let t=!1;for(let n of e)Rn(i,n)&1&&(t=!0);return t}function Md(i,e,t){let n=t.map(l=>i[l.id]),s=t.map(l=>l.type),r=n.filter(l=>!(l&1)),o=i[e.id]>>1;function a(l){let h=[];for(let c=0;c<n.length;c++){let d=Ni(l,n[c]);if(s[c]==2)for(let u of d)h.push(u);else h.push(d)}return e.combine(h)}return{create(l){for(let h of n)Rn(l,h);return l.values[o]=a(l),1},update(l,h){if(!hr(l,r))return 0;let c=a(l);return e.compare(c,l.values[o])?0:(l.values[o]=c,1)},reconfigure(l,h){let c=hr(l,n),d=h.config.facets[e.id],u=h.facet(e);if(d&&!c&&Jr(t,d))return l.values[o]=u,0;let f=a(l);return e.compare(f,u)?(l.values[o]=u,0):(l.values[o]=f,1)}}}const ii=D.define({static:!0});class me{constructor(e,t,n,s,r){this.id=e,this.createF=t,this.updateF=n,this.compareF=s,this.spec=r,this.provides=void 0}static define(e){let t=new me(Yr++,e.create,e.update,e.compare||((n,s)=>n===s),e);return e.provide&&(t.provides=e.provide(t)),t}create(e){let t=e.facet(ii).find(n=>n.field==this);return((t==null?void 0:t.create)||this.createF)(e)}slot(e){let t=e[this.id]>>1;return{create:n=>(n.values[t]=this.create(n),1),update:(n,s)=>{let r=n.values[t],o=this.updateF(r,s);return this.compareF(r,o)?0:(n.values[t]=o,1)},reconfigure:(n,s)=>{let r=n.facet(ii),o=s.facet(ii),a;return(a=r.find(l=>l.field==this))&&a!=o.find(l=>l.field==this)?(n.values[t]=a.create(n),1):s.config.address[this.id]!=null?(n.values[t]=s.field(this),0):(n.values[t]=this.create(n),1)}}}init(e){return[this,ii.of({field:this,create:e})]}get extension(){return this}}const _t={lowest:4,low:3,default:2,high:1,highest:0};function xn(i){return e=>new xl(e,i)}const pn={highest:xn(_t.highest),high:xn(_t.high),default:xn(_t.default),low:xn(_t.low),lowest:xn(_t.lowest)};class xl{constructor(e,t){this.inner=e,this.prec=t}get extension(){return this}}class ls{of(e){return new cr(this,e)}reconfigure(e){return ls.reconfigure.of({compartment:this,extension:e})}get(e){return e.config.compartments.get(this)}}class cr{constructor(e,t){this.compartment=e,this.inner=t}get extension(){return this}}class Pi{constructor(e,t,n,s,r,o){for(this.base=e,this.compartments=t,this.dynamicSlots=n,this.address=s,this.staticValues=r,this.facets=o,this.statusTemplate=[];this.statusTemplate.length<n.length;)this.statusTemplate.push(0)}staticFacet(e){let t=this.address[e.id];return t==null?e.default:this.staticValues[t>>1]}static resolve(e,t,n){let s=[],r=Object.create(null),o=new Map;for(let u of Od(e,t,o))u instanceof me?s.push(u):(r[u.facet.id]||(r[u.facet.id]=[])).push(u);let a=Object.create(null),l=[],h=[];for(let u of s)a[u.id]=h.length<<1,h.push(f=>u.slot(f));let c=n==null?void 0:n.config.facets;for(let u in r){let f=r[u],p=f[0].facet,m=c&&c[u]||[];if(f.every(g=>g.type==0))if(a[p.id]=l.length<<1|1,Jr(m,f))l.push(n.facet(p));else{let g=p.combine(f.map(y=>y.value));l.push(n&&p.compare(g,n.facet(p))?n.facet(p):g)}else{for(let g of f)g.type==0?(a[g.id]=l.length<<1|1,l.push(g.value)):(a[g.id]=h.length<<1,h.push(y=>g.dynamicSlot(y)));a[p.id]=h.length<<1,h.push(g=>Md(g,p,f))}}let d=h.map(u=>u(a));return new Pi(e,o,d,a,l,r)}}function Od(i,e,t){let n=[[],[],[],[],[]],s=new Map;function r(o,a){let l=s.get(o);if(l!=null){if(l<=a)return;let h=n[l].indexOf(o);h>-1&&n[l].splice(h,1),o instanceof cr&&t.delete(o.compartment)}if(s.set(o,a),Array.isArray(o))for(let h of o)r(h,a);else if(o instanceof cr){if(t.has(o.compartment))throw new RangeError("Duplicate use of compartment in extensions");let h=e.get(o.compartment)||o.inner;t.set(o.compartment,h),r(h,a)}else if(o instanceof xl)r(o.inner,o.prec);else if(o instanceof me)n[a].push(o),o.provides&&r(o.provides,a);else if(o instanceof Oi)n[a].push(o),o.facet.extensions&&r(o.facet.extensions,_t.default);else{let h=o.extension;if(!h)throw new Error(`Unrecognized extension value in extension set (${o}).`);if(h==o)throw new Error(`Unrecognized extension value in extension set (${o}). This sometimes happens because multiple instances of @codemirror/state are loaded, breaking instanceof checks.`);r(h,a)}}return r(i,_t.default),n.reduce((o,a)=>o.concat(a))}function Rn(i,e){if(e&1)return 2;let t=e>>1,n=i.status[t];if(n==4)throw new Error("Cyclic dependency between fields and/or facets");if(n&2)return n;i.status[t]=4;let s=i.computeSlot(i,i.config.dynamicSlots[t]);return i.status[t]=2|s}function Ni(i,e){return e&1?i.config.staticValues[e>>1]:i.values[e>>1]}const Sl=D.define(),dr=D.define({combine:i=>i.some(e=>e),static:!0}),Cl=D.define({combine:i=>i.length?i[0]:void 0,static:!0}),Al=D.define(),Tl=D.define(),Ml=D.define(),Ol=D.define({combine:i=>i.length?i[0]:!1});class yt{constructor(e,t){this.type=e,this.value=t}static define(){return new Id}}class Id{of(e){return new yt(this,e)}}class Dd{constructor(e){this.map=e}of(e){return new N(this,e)}}class N{constructor(e,t){this.type=e,this.value=t}map(e){let t=this.type.map(this.value,e);return t===void 0?void 0:t==this.value?this:new N(this.type,t)}is(e){return this.type==e}static define(e={}){return new Dd(e.map||(t=>t))}static mapEffects(e,t){if(!e.length)return e;let n=[];for(let s of e){let r=s.map(t);r&&n.push(r)}return n}}N.reconfigure=N.define();N.appendConfig=N.define();class ae{constructor(e,t,n,s,r,o){this.startState=e,this.changes=t,this.selection=n,this.effects=s,this.annotations=r,this.scrollIntoView=o,this._doc=null,this._state=null,n&&kl(n,t.newLength),r.some(a=>a.type==ae.time)||(this.annotations=r.concat(ae.time.of(Date.now())))}static create(e,t,n,s,r,o){return new ae(e,t,n,s,r,o)}get newDoc(){return this._doc||(this._doc=this.changes.apply(this.startState.doc))}get newSelection(){return this.selection||this.startState.selection.map(this.changes)}get state(){return this._state||this.startState.applyTransaction(this),this._state}annotation(e){for(let t of this.annotations)if(t.type==e)return t.value}get docChanged(){return!this.changes.empty}get reconfigured(){return this.startState.config!=this.state.config}isUserEvent(e){let t=this.annotation(ae.userEvent);return!!(t&&(t==e||t.length>e.length&&t.slice(0,e.length)==e&&t[e.length]=="."))}}ae.time=yt.define();ae.userEvent=yt.define();ae.addToHistory=yt.define();ae.remote=yt.define();function Rd(i,e){let t=[];for(let n=0,s=0;;){let r,o;if(n<i.length&&(s==e.length||e[s]>=i[n]))r=i[n++],o=i[n++];else if(s<e.length)r=e[s++],o=e[s++];else return t;!t.length||t[t.length-1]<r?t.push(r,o):t[t.length-1]<o&&(t[t.length-1]=o)}}function Il(i,e,t){var n;let s,r,o;return t?(s=e.changes,r=oe.empty(e.changes.length),o=i.changes.compose(e.changes)):(s=e.changes.map(i.changes),r=i.changes.mapDesc(e.changes,!0),o=i.changes.compose(s)),{changes:o,selection:e.selection?e.selection.map(r):(n=i.selection)===null||n===void 0?void 0:n.map(s),effects:N.mapEffects(i.effects,s).concat(N.mapEffects(e.effects,r)),annotations:i.annotations.length?i.annotations.concat(e.annotations):e.annotations,scrollIntoView:i.scrollIntoView||e.scrollIntoView}}function ur(i,e,t){let n=e.selection,s=Qt(e.annotations);return e.userEvent&&(s=s.concat(ae.userEvent.of(e.userEvent))),{changes:e.changes instanceof oe?e.changes:oe.of(e.changes||[],t,i.facet(Cl)),selection:n&&(n instanceof b?n:b.single(n.anchor,n.head)),effects:Qt(e.effects),annotations:s,scrollIntoView:!!e.scrollIntoView}}function Dl(i,e,t){let n=ur(i,e.length?e[0]:{},i.doc.length);e.length&&e[0].filter===!1&&(t=!1);for(let r=1;r<e.length;r++){e[r].filter===!1&&(t=!1);let o=!!e[r].sequential;n=Il(n,ur(i,e[r],o?n.changes.newLength:i.doc.length),o)}let s=ae.create(i,n.changes,n.selection,n.effects,n.annotations,n.scrollIntoView);return Ld(t?Ed(s):s)}function Ed(i){let e=i.startState,t=!0;for(let s of e.facet(Al)){let r=s(i);if(r===!1){t=!1;break}Array.isArray(r)&&(t=t===!0?r:Rd(t,r))}if(t!==!0){let s,r;if(t===!1)r=i.changes.invertedDesc,s=oe.empty(e.doc.length);else{let o=i.changes.filter(t);s=o.changes,r=o.filtered.mapDesc(o.changes).invertedDesc}i=ae.create(e,s,i.selection&&i.selection.map(r),N.mapEffects(i.effects,r),i.annotations,i.scrollIntoView)}let n=e.facet(Tl);for(let s=n.length-1;s>=0;s--){let r=n[s](i);r instanceof ae?i=r:Array.isArray(r)&&r.length==1&&r[0]instanceof ae?i=r[0]:i=Dl(e,Qt(r),!1)}return i}function Ld(i){let e=i.startState,t=e.facet(Ml),n=i;for(let s=t.length-1;s>=0;s--){let r=t[s](i);r&&Object.keys(r).length&&(n=Il(n,ur(e,r,i.changes.newLength),!0))}return n==i?i:ae.create(e,i.changes,i.selection,n.effects,n.annotations,n.scrollIntoView)}const Bd=[];function Qt(i){return i==null?Bd:Array.isArray(i)?i:[i]}var Z=function(i){return i[i.Word=0]="Word",i[i.Space=1]="Space",i[i.Other=2]="Other",i}(Z||(Z={}));const _d=/[\u00df\u0587\u0590-\u05f4\u0600-\u06ff\u3040-\u309f\u30a0-\u30ff\u3400-\u4db5\u4e00-\u9fcc\uac00-\ud7af]/;let fr;try{fr=new RegExp("[\\p{Alphabetic}\\p{Number}_]","u")}catch{}function Pd(i){if(fr)return fr.test(i);for(let e=0;e<i.length;e++){let t=i[e];if(/\w/.test(t)||t>""&&(t.toUpperCase()!=t.toLowerCase()||_d.test(t)))return!0}return!1}function Nd(i){return e=>{if(!/\S/.test(e))return Z.Space;if(Pd(e))return Z.Word;for(let t=0;t<i.length;t++)if(e.indexOf(i[t])>-1)return Z.Word;return Z.Other}}class z{constructor(e,t,n,s,r,o){this.config=e,this.doc=t,this.selection=n,this.values=s,this.status=e.statusTemplate.slice(),this.computeSlot=r,o&&(o._state=this);for(let a=0;a<this.config.dynamicSlots.length;a++)Rn(this,a<<1);this.computeSlot=null}field(e,t=!0){let n=this.config.address[e.id];if(n==null){if(t)throw new RangeError("Field is not present in this state");return}return Rn(this,n),Ni(this,n)}update(...e){return Dl(this,e,!0)}applyTransaction(e){let t=this.config,{base:n,compartments:s}=t;for(let a of e.effects)a.is(ls.reconfigure)?(t&&(s=new Map,t.compartments.forEach((l,h)=>s.set(h,l)),t=null),s.set(a.value.compartment,a.value.extension)):a.is(N.reconfigure)?(t=null,n=a.value):a.is(N.appendConfig)&&(t=null,n=Qt(n).concat(a.value));let r;t?r=e.startState.values.slice():(t=Pi.resolve(n,s,this),r=new z(t,this.doc,this.selection,t.dynamicSlots.map(()=>null),(l,h)=>h.reconfigure(l,this),null).values);let o=e.startState.facet(dr)?e.newSelection:e.newSelection.asSingle();new z(t,e.newDoc,o,r,(a,l)=>l.update(a,e),e)}replaceSelection(e){return typeof e=="string"&&(e=this.toText(e)),this.changeByRange(t=>({changes:{from:t.from,to:t.to,insert:e},range:b.cursor(t.from+e.length,-1)}))}changeByRange(e){let t=this.selection,n=e(t.ranges[0]),s=this.changes(n.changes),r=[n.range],o=Qt(n.effects);for(let a=1;a<t.ranges.length;a++){let l=e(t.ranges[a]),h=this.changes(l.changes),c=h.map(s);for(let u=0;u<a;u++)r[u]=r[u].map(c);let d=s.mapDesc(h,!0);r.push(l.range.map(d)),s=s.compose(c),o=N.mapEffects(o,c).concat(N.mapEffects(Qt(l.effects),d))}return{changes:s,selection:b.create(r,t.mainIndex),effects:o}}changes(e=[]){return e instanceof oe?e:oe.of(e,this.doc.length,this.facet(z.lineSeparator))}toText(e){return j.of(e.split(this.facet(z.lineSeparator)||or))}sliceDoc(e=0,t=this.doc.length){return this.doc.sliceString(e,t,this.lineBreak)}facet(e){let t=this.config.address[e.id];return t==null?e.default:(Rn(this,t),Ni(this,t))}toJSON(e){let t={doc:this.sliceDoc(),selection:this.selection.toJSON()};if(e)for(let n in e){let s=e[n];s instanceof me&&this.config.address[s.id]!=null&&(t[n]=s.spec.toJSON(this.field(e[n]),this))}return t}static fromJSON(e,t={},n){if(!e||typeof e.doc!="string")throw new RangeError("Invalid JSON representation for EditorState");let s=[];if(n){for(let r in n)if(Object.prototype.hasOwnProperty.call(e,r)){let o=n[r],a=e[r];s.push(o.init(l=>o.spec.fromJSON(a,l)))}}return z.create({doc:e.doc,selection:b.fromJSON(e.selection),extensions:t.extensions?s.concat([t.extensions]):s})}static create(e={}){let t=Pi.resolve(e.extensions||[],new Map),n=e.doc instanceof j?e.doc:j.of((e.doc||"").split(t.staticFacet(z.lineSeparator)||or)),s=e.selection?e.selection instanceof b?e.selection:b.single(e.selection.anchor,e.selection.head):b.single(0);return kl(s,n.length),t.staticFacet(dr)||(s=s.asSingle()),new z(t,n,s,t.dynamicSlots.map(()=>null),(r,o)=>o.create(r),null)}get tabSize(){return this.facet(z.tabSize)}get lineBreak(){return this.facet(z.lineSeparator)||`
`}get readOnly(){return this.facet(Ol)}phrase(e,...t){for(let n of this.facet(z.phrases))if(Object.prototype.hasOwnProperty.call(n,e)){e=n[e];break}return t.length&&(e=e.replace(/\$(\$|\d*)/g,(n,s)=>{if(s=="$")return"$";let r=+(s||1);return!r||r>t.length?n:t[r-1]})),e}languageDataAt(e,t,n=-1){let s=[];for(let r of this.facet(Sl))for(let o of r(this,t,n))Object.prototype.hasOwnProperty.call(o,e)&&s.push(o[e]);return s}charCategorizer(e){let t=this.languageDataAt("wordChars",e);return Nd(t.length?t[0]:"")}wordAt(e){let{text:t,from:n,length:s}=this.doc.lineAt(e),r=this.charCategorizer(e),o=e-n,a=e-n;for(;o>0;){let l=he(t,o,!1);if(r(t.slice(l,o))!=Z.Word)break;o=l}for(;a<s;){let l=he(t,a);if(r(t.slice(a,l))!=Z.Word)break;a=l}return o==a?null:b.range(o+n,a+n)}}z.allowMultipleSelections=dr;z.tabSize=D.define({combine:i=>i.length?i[0]:4});z.lineSeparator=Cl;z.readOnly=Ol;z.phrases=D.define({compare(i,e){let t=Object.keys(i),n=Object.keys(e);return t.length==n.length&&t.every(s=>i[s]==e[s])}});z.languageData=Sl;z.changeFilter=Al;z.transactionFilter=Tl;z.transactionExtender=Ml;ls.reconfigure=N.define();function bt(i,e,t={}){let n={};for(let s of i)for(let r of Object.keys(s)){let o=s[r],a=n[r];if(a===void 0)n[r]=o;else if(!(a===o||o===void 0))if(Object.hasOwnProperty.call(t,r))n[r]=t[r](a,o);else throw new Error("Config merge conflict for field "+r)}for(let s in e)n[s]===void 0&&(n[s]=e[s]);return n}class Mt{eq(e){return this==e}range(e,t=e){return pr.create(e,t,this)}}Mt.prototype.startSide=Mt.prototype.endSide=0;Mt.prototype.point=!1;Mt.prototype.mapMode=Se.TrackDel;function Xr(i,e){return i==e||i.constructor==e.constructor&&i.eq(e)}let pr=class Rl{constructor(e,t,n){this.from=e,this.to=t,this.value=n}static create(e,t,n){return new Rl(e,t,n)}};function mr(i,e){return i.from-e.from||i.value.startSide-e.value.startSide}class Qr{constructor(e,t,n,s){this.from=e,this.to=t,this.value=n,this.maxPoint=s}get length(){return Ut(this.to)}findIndex(e,t,n,s=0){let r=n?this.to:this.from;for(let o=s,a=r.length;;){if(o==a)return o;let l=o+a>>1,h=r[l]-e||(n?this.value[l].endSide:this.value[l].startSide)-t;if(l==o)return h>=0?o:a;h>=0?a=l:o=l+1}}between(e,t,n,s){for(let r=this.findIndex(t,-1e9,!0),o=this.findIndex(n,1e9,!1,r);r<o;r++)if(s(this.from[r]+e,this.to[r]+e,this.value[r])===!1)return!1}map(e,t,n,s,r){let o=[],a=[],l=[],h=-1,c=-1;e:for(let d=0;d<this.value.length;d++){let u=this.value[d],f=this.from[d]+e,p=this.to[d]+e,m,g;if(f==p){let y=t.mapPos(f,u.startSide,u.mapMode);if(y==null||(m=g=y,u.startSide!=u.endSide&&(g=t.mapPos(f,u.endSide),g<m)))continue}else if(m=t.mapPos(f,u.startSide),g=t.mapPos(p,u.endSide),m>g||m==g&&u.startSide>0&&u.endSide<=0)continue;if(!((g-m||u.endSide-u.startSide)<0))if(h<0&&(h=m),u.point&&(c=Math.max(c,g-m)),(m-n||u.startSide-s)>=0)o.push(u),a.push(m-h),l.push(g-h),n=g,s=u.endSide;else{if(m==g)for(let y=o.length;y>0;y--){if((m-(l[y-1]+h)||u.startSide-o[y-1].endSide)>=0){o.splice(y,0,u),a.splice(y,0,m-h),l.splice(y,0,g-h);continue e}if((m-(a[y-1]+h)||u.endSide-o[y-1].startSide)>0)break}r(m,g,u)}}return{mapped:o.length?new Qr(a,l,o,c):null,pos:h}}}class H{constructor(e,t,n,s){this.chunkPos=e,this.chunk=t,this.nextLayer=n,this.maxPoint=s}static create(e,t,n,s){return new H(e,t,n,s)}get length(){let e=this.chunk.length-1;return e<0?0:Math.max(this.chunkEnd(e),this.nextLayer.length)}get size(){if(this.isEmpty)return 0;let e=this.nextLayer.size;for(let t of this.chunk)e+=t.value.length;return e}chunkEnd(e){return this.chunkPos[e]+this.chunk[e].length}update(e){let{add:t=[],sort:n=!1,filterFrom:s=0,filterTo:r=this.length}=e,o=e.filter;if(t.length==0&&!o)return this;if(n&&(t=t.slice().sort(mr)),this.isEmpty)return t.length?H.of(t):this;let a=new El(this,null,-1).goto(0),l=0,h=[],c=new At;for(;a.value||l<t.length;)if(l<t.length&&(a.from-t[l].from||a.startSide-t[l].value.startSide)>=0){let d=t[l++];c.addInner(d.from,d.to,d.value,!1)||h.push(d)}else a.rangeIndex==1&&a.chunkIndex<this.chunk.length&&(l==t.length||this.chunkEnd(a.chunkIndex)<t[l].from)&&(!o||s>this.chunkEnd(a.chunkIndex)||r<this.chunkPos[a.chunkIndex])&&c.addChunk(this.chunkPos[a.chunkIndex],this.chunk[a.chunkIndex])?a.nextChunk():((!o||s>a.to||r<a.from||o(a.from,a.to,a.value))&&(c.addInner(a.from,a.to,a.value,!1)||h.push(pr.create(a.from,a.to,a.value))),a.next());return c.finishInner(this.nextLayer.isEmpty&&!h.length?H.empty:this.nextLayer.update({add:h,filter:o,filterFrom:s,filterTo:r}))}map(e){if(e.empty||this.isEmpty)return this;let t=[],n=[],s=-1,r,o=(l,h,c)=>{r||(r=new At),r.addRange(l,h,c,!1)};for(let l=0;l<this.chunk.length;l++){let h=this.chunkPos[l],c=this.chunk[l],d=e.touchesRange(h,h+c.length);if(d===!1)s=Math.max(s,c.maxPoint),t.push(c),n.push(e.mapPos(h));else if(d===!0){let[u,f]=t.length?[Ut(n)+Ut(t).length,Ut(Ut(t).value).endSide]:[-1,-1],{mapped:p,pos:m}=c.map(h,e,u,f,o);p&&(s=Math.max(s,p.maxPoint),t.push(p),n.push(m))}}let a=this.nextLayer.map(e);return r&&(a=r.finishInner(a)),t.length==0?a:new H(n,t,a||H.empty,s)}between(e,t,n){if(!this.isEmpty){for(let s=0;s<this.chunk.length;s++){let r=this.chunkPos[s],o=this.chunk[s];if(t>=r&&e<=r+o.length&&o.between(r,e-r,t-r,n)===!1)return}this.nextLayer.between(e,t,n)}}iter(e=0){return Hn.from([this]).goto(e)}get isEmpty(){return this.nextLayer==this}static iter(e,t=0){return Hn.from(e).goto(t)}static compare(e,t,n,s,r=-1){let o=e.filter(d=>d.maxPoint>0||!d.isEmpty&&d.maxPoint>=r),a=t.filter(d=>d.maxPoint>0||!d.isEmpty&&d.maxPoint>=r),l=qo(o,a,n),h=new Sn(o,l,r),c=new Sn(a,l,r);n.iterGaps((d,u,f)=>Ho(h,d,c,u,f,s)),n.empty&&n.length==0&&Ho(h,0,c,0,0,s)}static eq(e,t,n=0,s){s==null&&(s=999999999);let r=e.filter(c=>!c.isEmpty&&t.indexOf(c)<0),o=t.filter(c=>!c.isEmpty&&e.indexOf(c)<0);if(r.length!=o.length)return!1;if(!r.length)return!0;let a=qo(r,o),l=new Sn(r,a,0).goto(n),h=new Sn(o,a,0).goto(n);for(;;){if(l.to!=h.to||!gr(l.active,h.active)||l.point&&(!h.point||!Xr(l.point,h.point)))return!1;if(l.to>s)return!0;l.next(),h.next()}}static spans(e,t,n,s,r=-1){let o=new Sn(e,null,r).goto(t),a=t,l=o.openStart;for(;;){let h=Math.min(o.to,n);if(o.point){let c=o.activeForPoint(o.to),d=o.pointFrom<t?c.length+1:o.point.startSide<0?c.length:Math.min(c.length,l);s.point(a,h,o.point,c,d,o.pointRank),l=Math.min(o.openEnd(h),c.length)}else h>a&&(s.span(a,h,o.active,l),l=o.openEnd(h));if(o.to>n)return l+(o.point&&o.to>n?1:0);a=o.to,o.next()}}static of(e,t=!1){let n=new At;for(let s of e instanceof pr?[e]:t?Wd(e):e)n.add(s.from,s.to,s.value);return n.finish()}static join(e){if(!e.length)return H.empty;let t=Ut(e);for(let n=e.length-2;n>=0;n--)for(let s=e[n];s!=H.empty;s=s.nextLayer)t=new H(s.chunkPos,s.chunk,t,Math.max(s.maxPoint,t.maxPoint));return t}}H.empty=new H([],[],null,-1);function Ut(i){return i[i.length-1]}function Wd(i){if(i.length>1)for(let e=i[0],t=1;t<i.length;t++){let n=i[t];if(mr(e,n)>0)return i.slice().sort(mr);e=n}return i}H.empty.nextLayer=H.empty;class At{finishChunk(e){this.chunks.push(new Qr(this.from,this.to,this.value,this.maxPoint)),this.chunkPos.push(this.chunkStart),this.chunkStart=-1,this.setMaxPoint=Math.max(this.setMaxPoint,this.maxPoint),this.maxPoint=-1,e&&(this.from=[],this.to=[],this.value=[])}constructor(){this.chunks=[],this.chunkPos=[],this.chunkStart=-1,this.last=null,this.lastFrom=-1e9,this.lastTo=-1e9,this.from=[],this.to=[],this.value=[],this.maxPoint=-1,this.setMaxPoint=-1,this.nextLayer=null}add(e,t,n){this.addRange(e,t,n,!0)}addRange(e,t,n,s){this.addInner(e,t,n,s)||(this.nextLayer||(this.nextLayer=new At)).addRange(e,t,n,s)}addInner(e,t,n,s){let r=e-this.lastTo||n.startSide-this.last.endSide;if(s&&r<=0&&(e-this.lastFrom||n.startSide-this.last.startSide)<0)throw new Error("Ranges must be added sorted by `from` position and `startSide`");return r<0?!1:(this.from.length==250&&this.finishChunk(!0),this.chunkStart<0&&(this.chunkStart=e),this.from.push(e-this.chunkStart),this.to.push(t-this.chunkStart),this.last=n,this.lastFrom=e,this.lastTo=t,this.value.push(n),n.point&&(this.maxPoint=Math.max(this.maxPoint,t-e)),!0)}addChunk(e,t){if((e-this.lastTo||t.value[0].startSide-this.last.endSide)<0)return!1;this.from.length&&this.finishChunk(!0),this.setMaxPoint=Math.max(this.setMaxPoint,t.maxPoint),this.chunks.push(t),this.chunkPos.push(e);let n=t.value.length-1;return this.last=t.value[n],this.lastFrom=t.from[n]+e,this.lastTo=t.to[n]+e,!0}finish(){return this.finishInner(H.empty)}finishInner(e){if(this.from.length&&this.finishChunk(!1),this.chunks.length==0)return e;let t=H.create(this.chunkPos,this.chunks,this.nextLayer?this.nextLayer.finishInner(e):e,this.setMaxPoint);return this.from=null,t}}function qo(i,e,t){let n=new Map;for(let r of i)for(let o=0;o<r.chunk.length;o++)r.chunk[o].maxPoint<=0&&n.set(r.chunk[o],r.chunkPos[o]);let s=new Set;for(let r of e)for(let o=0;o<r.chunk.length;o++){let a=n.get(r.chunk[o]);a!=null&&(t?t.mapPos(a):a)==r.chunkPos[o]&&!(t!=null&&t.touchesRange(a,a+r.chunk[o].length))&&s.add(r.chunk[o])}return s}class El{constructor(e,t,n,s=0){this.layer=e,this.skip=t,this.minPoint=n,this.rank=s}get startSide(){return this.value?this.value.startSide:0}get endSide(){return this.value?this.value.endSide:0}goto(e,t=-1e9){return this.chunkIndex=this.rangeIndex=0,this.gotoInner(e,t,!1),this}gotoInner(e,t,n){for(;this.chunkIndex<this.layer.chunk.length;){let s=this.layer.chunk[this.chunkIndex];if(!(this.skip&&this.skip.has(s)||this.layer.chunkEnd(this.chunkIndex)<e||s.maxPoint<this.minPoint))break;this.chunkIndex++,n=!1}if(this.chunkIndex<this.layer.chunk.length){let s=this.layer.chunk[this.chunkIndex].findIndex(e-this.layer.chunkPos[this.chunkIndex],t,!0);(!n||this.rangeIndex<s)&&this.setRangeIndex(s)}this.next()}forward(e,t){(this.to-e||this.endSide-t)<0&&this.gotoInner(e,t,!0)}next(){for(;;)if(this.chunkIndex==this.layer.chunk.length){this.from=this.to=1e9,this.value=null;break}else{let e=this.layer.chunkPos[this.chunkIndex],t=this.layer.chunk[this.chunkIndex],n=e+t.from[this.rangeIndex];if(this.from=n,this.to=e+t.to[this.rangeIndex],this.value=t.value[this.rangeIndex],this.setRangeIndex(this.rangeIndex+1),this.minPoint<0||this.value.point&&this.to-this.from>=this.minPoint)break}}setRangeIndex(e){if(e==this.layer.chunk[this.chunkIndex].value.length){if(this.chunkIndex++,this.skip)for(;this.chunkIndex<this.layer.chunk.length&&this.skip.has(this.layer.chunk[this.chunkIndex]);)this.chunkIndex++;this.rangeIndex=0}else this.rangeIndex=e}nextChunk(){this.chunkIndex++,this.rangeIndex=0,this.next()}compare(e){return this.from-e.from||this.startSide-e.startSide||this.rank-e.rank||this.to-e.to||this.endSide-e.endSide}}class Hn{constructor(e){this.heap=e}static from(e,t=null,n=-1){let s=[];for(let r=0;r<e.length;r++)for(let o=e[r];!o.isEmpty;o=o.nextLayer)o.maxPoint>=n&&s.push(new El(o,t,n,r));return s.length==1?s[0]:new Hn(s)}get startSide(){return this.value?this.value.startSide:0}goto(e,t=-1e9){for(let n of this.heap)n.goto(e,t);for(let n=this.heap.length>>1;n>=0;n--)Bs(this.heap,n);return this.next(),this}forward(e,t){for(let n of this.heap)n.forward(e,t);for(let n=this.heap.length>>1;n>=0;n--)Bs(this.heap,n);(this.to-e||this.value.endSide-t)<0&&this.next()}next(){if(this.heap.length==0)this.from=this.to=1e9,this.value=null,this.rank=-1;else{let e=this.heap[0];this.from=e.from,this.to=e.to,this.value=e.value,this.rank=e.rank,e.value&&e.next(),Bs(this.heap,0)}}}function Bs(i,e){for(let t=i[e];;){let n=(e<<1)+1;if(n>=i.length)break;let s=i[n];if(n+1<i.length&&s.compare(i[n+1])>=0&&(s=i[n+1],n++),t.compare(s)<0)break;i[n]=t,i[e]=s,e=n}}class Sn{constructor(e,t,n){this.minPoint=n,this.active=[],this.activeTo=[],this.activeRank=[],this.minActive=-1,this.point=null,this.pointFrom=0,this.pointRank=0,this.to=-1e9,this.endSide=0,this.openStart=-1,this.cursor=Hn.from(e,t,n)}goto(e,t=-1e9){return this.cursor.goto(e,t),this.active.length=this.activeTo.length=this.activeRank.length=0,this.minActive=-1,this.to=e,this.endSide=t,this.openStart=-1,this.next(),this}forward(e,t){for(;this.minActive>-1&&(this.activeTo[this.minActive]-e||this.active[this.minActive].endSide-t)<0;)this.removeActive(this.minActive);this.cursor.forward(e,t)}removeActive(e){si(this.active,e),si(this.activeTo,e),si(this.activeRank,e),this.minActive=Fo(this.active,this.activeTo)}addActive(e){let t=0,{value:n,to:s,rank:r}=this.cursor;for(;t<this.activeRank.length&&(r-this.activeRank[t]||s-this.activeTo[t])>0;)t++;ri(this.active,t,n),ri(this.activeTo,t,s),ri(this.activeRank,t,r),e&&ri(e,t,this.cursor.from),this.minActive=Fo(this.active,this.activeTo)}next(){let e=this.to,t=this.point;this.point=null;let n=this.openStart<0?[]:null;for(;;){let s=this.minActive;if(s>-1&&(this.activeTo[s]-this.cursor.from||this.active[s].endSide-this.cursor.startSide)<0){if(this.activeTo[s]>e){this.to=this.activeTo[s],this.endSide=this.active[s].endSide;break}this.removeActive(s),n&&si(n,s)}else if(this.cursor.value)if(this.cursor.from>e){this.to=this.cursor.from,this.endSide=this.cursor.startSide;break}else{let r=this.cursor.value;if(!r.point)this.addActive(n),this.cursor.next();else if(t&&this.cursor.to==this.to&&this.cursor.from<this.cursor.to)this.cursor.next();else{this.point=r,this.pointFrom=this.cursor.from,this.pointRank=this.cursor.rank,this.to=this.cursor.to,this.endSide=r.endSide,this.cursor.next(),this.forward(this.to,this.endSide);break}}else{this.to=this.endSide=1e9;break}}if(n){this.openStart=0;for(let s=n.length-1;s>=0&&n[s]<e;s--)this.openStart++}}activeForPoint(e){if(!this.active.length)return this.active;let t=[];for(let n=this.active.length-1;n>=0&&!(this.activeRank[n]<this.pointRank);n--)(this.activeTo[n]>e||this.activeTo[n]==e&&this.active[n].endSide>=this.point.endSide)&&t.push(this.active[n]);return t.reverse()}openEnd(e){let t=0;for(let n=this.activeTo.length-1;n>=0&&this.activeTo[n]>e;n--)t++;return t}}function Ho(i,e,t,n,s,r){i.goto(e),t.goto(n);let o=n+s,a=n,l=n-e,h=!!r.boundChange;for(let c=!1;;){let d=i.to+l-t.to,u=d||i.endSide-t.endSide,f=u<0?i.to+l:t.to,p=Math.min(f,o);if(i.point||t.point?(i.point&&t.point&&Xr(i.point,t.point)&&gr(i.activeForPoint(i.to),t.activeForPoint(t.to))||r.comparePoint(a,p,i.point,t.point),c=!1):(c&&(r.boundChange(a),c=!1),p>a&&!gr(i.active,t.active)&&r.compareRange(a,p,i.active,t.active),h&&p<o&&(d||i.openEnd(f)!=t.openEnd(f))&&(c=!0)),f>o)break;a=f,u<=0&&i.next(),u>=0&&t.next()}}function gr(i,e){if(i.length!=e.length)return!1;for(let t=0;t<i.length;t++)if(i[t]!=e[t]&&!Xr(i[t],e[t]))return!1;return!0}function si(i,e){for(let t=e,n=i.length-1;t<n;t++)i[t]=i[t+1];i.pop()}function ri(i,e,t){for(let n=i.length-1;n>=e;n--)i[n+1]=i[n];i[e]=t}function Fo(i,e){let t=-1,n=1e9;for(let s=0;s<e.length;s++)(e[s]-n||i[s].endSide-i[t].endSide)<0&&(t=s,n=e[s]);return t}function hs(i,e,t=i.length){let n=0;for(let s=0;s<t&&s<i.length;)i.charCodeAt(s)==9?(n+=e-n%e,s++):(n++,s=he(i,s));return n}function qd(i,e,t,n){for(let s=0,r=0;;){if(r>=e)return s;if(s==i.length)break;r+=i.charCodeAt(s)==9?t-r%t:1,s=he(i,s)}return i.length}const yr="ͼ",Vo=typeof Symbol>"u"?"__"+yr:Symbol.for(yr),br=typeof Symbol>"u"?"__styleSet"+Math.floor(Math.random()*1e8):Symbol("styleSet"),zo=typeof globalThis<"u"?globalThis:typeof window<"u"?window:{};class on{constructor(e,t){this.rules=[];let{finish:n}=t||{};function s(o){return/^@/.test(o)?[o]:o.split(/,\s*/)}function r(o,a,l,h){let c=[],d=/^@(\w+)\b/.exec(o[0]),u=d&&d[1]=="keyframes";if(d&&a==null)return l.push(o[0]+";");for(let f in a){let p=a[f];if(/&/.test(f))r(f.split(/,\s*/).map(m=>o.map(g=>m.replace(/&/,g))).reduce((m,g)=>m.concat(g)),p,l);else if(p&&typeof p=="object"){if(!d)throw new RangeError("The value of a property ("+f+") should be a primitive value.");r(s(f),p,c,u)}else p!=null&&c.push(f.replace(/_.*/,"").replace(/[A-Z]/g,m=>"-"+m.toLowerCase())+": "+p+";")}(c.length||u)&&l.push((n&&!d&&!h?o.map(n):o).join(", ")+" {"+c.join(" ")+"}")}for(let o in e)r(s(o),e[o],this.rules)}getRules(){return this.rules.join(`
`)}static newName(){let e=zo[Vo]||1;return zo[Vo]=e+1,yr+e.toString(36)}static mount(e,t,n){let s=e[br],r=n&&n.nonce;s?r&&s.setNonce(r):s=new Hd(e,r),s.mount(Array.isArray(t)?t:[t],e)}}let jo=new Map;class Hd{constructor(e,t){let n=e.ownerDocument||e,s=n.defaultView;if(!e.head&&e.adoptedStyleSheets&&s.CSSStyleSheet){let r=jo.get(n);if(r)return e[br]=r;this.sheet=new s.CSSStyleSheet,jo.set(n,this)}else this.styleTag=n.createElement("style"),t&&this.styleTag.setAttribute("nonce",t);this.modules=[],e[br]=this}mount(e,t){let n=this.sheet,s=0,r=0,o=!1;for(let a=0;a<e.length;a++){let l=e[a],h=this.modules.indexOf(l);if(h<r&&h>-1&&(this.modules.splice(h,1),o=!0,r--,h=-1),h==-1){if(this.modules.splice(r++,0,l),o=!0,n)for(let c=0;c<l.rules.length;c++)n.insertRule(l.rules[c],s++)}else{for(;r<h;)s+=this.modules[r++].rules.length;s+=l.rules.length,r++}}if(n)t.adoptedStyleSheets.indexOf(this.sheet)<0&&(t.adoptedStyleSheets=[this.sheet,...t.adoptedStyleSheets]);else{if(o){let l="";for(let h=0;h<this.modules.length;h++)l+=this.modules[h].getRules()+`
`;this.styleTag.textContent=l}let a=t.head||t;this.styleTag.parentNode!=a&&a.insertBefore(this.styleTag,a.firstChild)}}setNonce(e){this.styleTag&&this.styleTag.getAttribute("nonce")!=e&&this.styleTag.setAttribute("nonce",e)}}var Ot={8:"Backspace",9:"Tab",10:"Enter",12:"NumLock",13:"Enter",16:"Shift",17:"Control",18:"Alt",20:"CapsLock",27:"Escape",32:" ",33:"PageUp",34:"PageDown",35:"End",36:"Home",37:"ArrowLeft",38:"ArrowUp",39:"ArrowRight",40:"ArrowDown",44:"PrintScreen",45:"Insert",46:"Delete",59:";",61:"=",91:"Meta",92:"Meta",106:"*",107:"+",108:",",109:"-",110:".",111:"/",144:"NumLock",145:"ScrollLock",160:"Shift",161:"Shift",162:"Control",163:"Control",164:"Alt",165:"Alt",173:"-",186:";",187:"=",188:",",189:"-",190:".",191:"/",192:"`",219:"[",220:"\\",221:"]",222:"'"},Fn={48:")",49:"!",50:"@",51:"#",52:"$",53:"%",54:"^",55:"&",56:"*",57:"(",59:":",61:"+",173:"_",186:":",187:"+",188:"<",189:"_",190:">",191:"?",192:"~",219:"{",220:"|",221:"}",222:'"'},Fd=typeof navigator<"u"&&/Mac/.test(navigator.platform),Vd=typeof navigator<"u"&&/MSIE \d|Trident\/(?:[7-9]|\d{2,})\..*rv:(\d+)/.exec(navigator.userAgent);for(var ue=0;ue<10;ue++)Ot[48+ue]=Ot[96+ue]=String(ue);for(var ue=1;ue<=24;ue++)Ot[ue+111]="F"+ue;for(var ue=65;ue<=90;ue++)Ot[ue]=String.fromCharCode(ue+32),Fn[ue]=String.fromCharCode(ue);for(var _s in Ot)Fn.hasOwnProperty(_s)||(Fn[_s]=Ot[_s]);function zd(i){var e=Fd&&i.metaKey&&i.shiftKey&&!i.ctrlKey&&!i.altKey||Vd&&i.shiftKey&&i.key&&i.key.length==1||i.key=="Unidentified",t=!e&&i.key||(i.shiftKey?Fn:Ot)[i.keyCode]||i.key||"Unidentified";return t=="Esc"&&(t="Escape"),t=="Del"&&(t="Delete"),t=="Left"&&(t="ArrowLeft"),t=="Up"&&(t="ArrowUp"),t=="Right"&&(t="ArrowRight"),t=="Down"&&(t="ArrowDown"),t}function G(){var i=arguments[0];typeof i=="string"&&(i=document.createElement(i));var e=1,t=arguments[1];if(t&&typeof t=="object"&&t.nodeType==null&&!Array.isArray(t)){for(var n in t)if(Object.prototype.hasOwnProperty.call(t,n)){var s=t[n];typeof s=="string"?i.setAttribute(n,s):s!=null&&(i[n]=s)}e++}for(;e<arguments.length;e++)Ll(i,arguments[e]);return i}function Ll(i,e){if(typeof e=="string")i.appendChild(document.createTextNode(e));else if(e!=null)if(e.nodeType!=null)i.appendChild(e);else if(Array.isArray(e))for(var t=0;t<e.length;t++)Ll(i,e[t]);else throw new RangeError("Unsupported child node: "+e)}let xe=typeof navigator<"u"?navigator:{userAgent:"",vendor:"",platform:""},wr=typeof document<"u"?document:{documentElement:{style:{}}};const vr=/Edge\/(\d+)/.exec(xe.userAgent),Bl=/MSIE \d/.test(xe.userAgent),kr=/Trident\/(?:[7-9]|\d{2,})\..*rv:(\d+)/.exec(xe.userAgent),cs=!!(Bl||kr||vr),Uo=!cs&&/gecko\/(\d+)/i.test(xe.userAgent),Ps=!cs&&/Chrome\/(\d+)/.exec(xe.userAgent),$o="webkitFontSmoothing"in wr.documentElement.style,xr=!cs&&/Apple Computer/.test(xe.vendor),Ko=xr&&(/Mobile\/\w+/.test(xe.userAgent)||xe.maxTouchPoints>2);var O={mac:Ko||/Mac/.test(xe.platform),windows:/Win/.test(xe.platform),linux:/Linux|X11/.test(xe.platform),ie:cs,ie_version:Bl?wr.documentMode||6:kr?+kr[1]:vr?+vr[1]:0,gecko:Uo,gecko_version:Uo?+(/Firefox\/(\d+)/.exec(xe.userAgent)||[0,0])[1]:0,chrome:!!Ps,chrome_version:Ps?+Ps[1]:0,ios:Ko,android:/Android\b/.test(xe.userAgent),webkit:$o,webkit_version:$o?+(/\bAppleWebKit\/(\d+)/.exec(xe.userAgent)||[0,0])[1]:0,safari:xr,safari_version:xr?+(/\bVersion\/(\d+(\.\d+)?)/.exec(xe.userAgent)||[0,0])[1]:0,tabSize:wr.documentElement.style.tabSize!=null?"tab-size":"-moz-tab-size"};function Zr(i,e){for(let t in i)t=="class"&&e.class?e.class+=" "+i.class:t=="style"&&e.style?e.style+=";"+i.style:e[t]=i[t];return e}const Wi=Object.create(null);function eo(i,e,t){if(i==e)return!0;i||(i=Wi),e||(e=Wi);let n=Object.keys(i),s=Object.keys(e);if(n.length-0!=s.length-0)return!1;for(let r of n)if(r!=t&&(s.indexOf(r)==-1||i[r]!==e[r]))return!1;return!0}function jd(i,e){for(let t=i.attributes.length-1;t>=0;t--){let n=i.attributes[t].name;e[n]==null&&i.removeAttribute(n)}for(let t in e){let n=e[t];t=="style"?i.style.cssText=n:i.getAttribute(t)!=n&&i.setAttribute(t,n)}}function Go(i,e,t){let n=!1;if(e)for(let s in e)t&&s in t||(n=!0,s=="style"?i.style.cssText="":i.removeAttribute(s));if(t)for(let s in t)e&&e[s]==t[s]||(n=!0,s=="style"?i.style.cssText=t[s]:i.setAttribute(s,t[s]));return n}function Ud(i){let e=Object.create(null);for(let t=0;t<i.attributes.length;t++){let n=i.attributes[t];e[n.name]=n.value}return e}class Xn{eq(e){return!1}updateDOM(e,t,n){return!1}compare(e){return this==e||this.constructor==e.constructor&&this.eq(e)}get estimatedHeight(){return-1}get lineBreaks(){return 0}ignoreEvent(e){return!0}coordsAt(e,t,n){return null}get isHidden(){return!1}get editable(){return!1}destroy(e){}}var pe=function(i){return i[i.Text=0]="Text",i[i.WidgetBefore=1]="WidgetBefore",i[i.WidgetAfter=2]="WidgetAfter",i[i.WidgetRange=3]="WidgetRange",i}(pe||(pe={}));class _ extends Mt{constructor(e,t,n,s){super(),this.startSide=e,this.endSide=t,this.widget=n,this.spec=s}get heightRelevant(){return!1}static mark(e){return new Qn(e)}static widget(e){let t=Math.max(-1e4,Math.min(1e4,e.side||0)),n=!!e.block;return t+=n&&!e.inlineOrder?t>0?3e8:-4e8:t>0?1e8:-1e8,new Vt(e,t,t,n,e.widget||null,!1)}static replace(e){let t=!!e.block,n,s;if(e.isBlockGap)n=-5e8,s=4e8;else{let{start:r,end:o}=_l(e,t);n=(r?t?-3e8:-1:5e8)-1,s=(o?t?2e8:1:-6e8)+1}return new Vt(e,n,s,t,e.widget||null,!0)}static line(e){return new Zn(e)}static set(e,t=!1){return H.of(e,t)}hasHeight(){return this.widget?this.widget.estimatedHeight>-1:!1}}_.none=H.empty;class Qn extends _{constructor(e){let{start:t,end:n}=_l(e);super(t?-1:5e8,n?1:-6e8,null,e),this.tagName=e.tagName||"span",this.attrs=e.class&&e.attributes?Zr(e.attributes,{class:e.class}):e.class?{class:e.class}:e.attributes||Wi}eq(e){return this==e||e instanceof Qn&&this.tagName==e.tagName&&eo(this.attrs,e.attrs)}range(e,t=e){if(e>=t)throw new RangeError("Mark decorations may not be empty");return super.range(e,t)}}Qn.prototype.point=!1;class Zn extends _{constructor(e){super(-2e8,-2e8,null,e)}eq(e){return e instanceof Zn&&this.spec.class==e.spec.class&&eo(this.spec.attributes,e.spec.attributes)}range(e,t=e){if(t!=e)throw new RangeError("Line decoration ranges must be zero-length");return super.range(e,t)}}Zn.prototype.mapMode=Se.TrackBefore;Zn.prototype.point=!0;class Vt extends _{constructor(e,t,n,s,r,o){super(t,n,r,e),this.block=s,this.isReplace=o,this.mapMode=s?t<=0?Se.TrackBefore:Se.TrackAfter:Se.TrackDel}get type(){return this.startSide!=this.endSide?pe.WidgetRange:this.startSide<=0?pe.WidgetBefore:pe.WidgetAfter}get heightRelevant(){return this.block||!!this.widget&&(this.widget.estimatedHeight>=5||this.widget.lineBreaks>0)}eq(e){return e instanceof Vt&&$d(this.widget,e.widget)&&this.block==e.block&&this.startSide==e.startSide&&this.endSide==e.endSide}range(e,t=e){if(this.isReplace&&(e>t||e==t&&this.startSide>0&&this.endSide<=0))throw new RangeError("Invalid range for replacement decoration");if(!this.isReplace&&t!=e)throw new RangeError("Widget decorations can only have zero-length ranges");return super.range(e,t)}}Vt.prototype.point=!0;function _l(i,e=!1){let{inclusiveStart:t,inclusiveEnd:n}=i;return t==null&&(t=i.inclusive),n==null&&(n=i.inclusive),{start:t??e,end:n??e}}function $d(i,e){return i==e||!!(i&&e&&i.compare(e))}function Zt(i,e,t,n=0){let s=t.length-1;s>=0&&t[s]+n>=i?t[s]=Math.max(t[s],e):t.push(i,e)}class Vn extends Mt{constructor(e,t,n){super(),this.tagName=e,this.attributes=t,this.rank=n}eq(e){return e==this||e instanceof Vn&&this.tagName==e.tagName&&eo(this.attributes,e.attributes)}static create(e){return new Vn(e.tagName,e.attributes||Wi,e.rank==null?50:Math.max(0,Math.min(e.rank,100)))}static set(e,t=!1){return H.of(e,t)}}Vn.prototype.startSide=Vn.prototype.endSide=-1;function zn(i){let e;return i.nodeType==11?e=i.getSelection?i:i.ownerDocument:e=i,e.getSelection()}function Sr(i,e){return e?i==e||i.contains(e.nodeType!=1?e.parentNode:e):!1}function En(i,e){if(!e.anchorNode)return!1;try{return Sr(i,e.anchorNode)}catch{return!1}}function Ii(i){return i.nodeType==3?jn(i,0,i.nodeValue.length).getClientRects():i.nodeType==1?i.getClientRects():[]}function Ln(i,e,t,n){return t?Yo(i,e,t,n,-1)||Yo(i,e,t,n,1):!1}function It(i){for(var e=0;;e++)if(i=i.previousSibling,!i)return e}function qi(i){return i.nodeType==1&&/^(DIV|P|LI|UL|OL|BLOCKQUOTE|DD|DT|H\d|SECTION|PRE)$/.test(i.nodeName)}function Yo(i,e,t,n,s){for(;;){if(i==t&&e==n)return!0;if(e==(s<0?0:mt(i))){if(i.nodeName=="DIV")return!1;let r=i.parentNode;if(!r||r.nodeType!=1)return!1;e=It(i)+(s<0?0:1),i=r}else if(i.nodeType==1){if(i=i.childNodes[e+(s<0?-1:0)],i.nodeType==1&&i.contentEditable=="false")return!1;e=s<0?mt(i):0}else return!1}}function mt(i){return i.nodeType==3?i.nodeValue.length:i.childNodes.length}function Hi(i,e){let{left:t,right:n}=i;if(t==n)return i;let s=e?t:n;return{left:s,right:s,top:i.top,bottom:i.bottom}}function Kd(i){let e=i.visualViewport;return e?{left:0,right:e.width,top:0,bottom:e.height}:{left:0,right:i.innerWidth,top:0,bottom:i.innerHeight}}function Pl(i,e){let t=e.width/i.offsetWidth,n=e.height/i.offsetHeight;return(t>.995&&t<1.005||!isFinite(t)||Math.abs(e.width-i.offsetWidth)<1)&&(t=1),(n>.995&&n<1.005||!isFinite(n)||Math.abs(e.height-i.offsetHeight)<1)&&(n=1),{scaleX:t,scaleY:n}}function Gd(i,e,t,n,s,r,o,a){let l=i.ownerDocument,h=l.defaultView||window;for(let c=i,d=!1;c&&!d;)if(c.nodeType==1){let u,f=c==l.body,p=1,m=1;if(f)u=Kd(h);else{if(/^(fixed|sticky)$/.test(getComputedStyle(c).position)&&(d=!0),c.scrollHeight<=c.clientHeight&&c.scrollWidth<=c.clientWidth){c=c.assignedSlot||c.parentNode;continue}let x=c.getBoundingClientRect();({scaleX:p,scaleY:m}=Pl(c,x)),u={left:x.left,right:x.left+c.clientWidth*p,top:x.top,bottom:x.top+c.clientHeight*m}}let g=0,y=0;if(s=="nearest")e.top<u.top+o?(y=e.top-(u.top+o),t>0&&e.bottom>u.bottom+y&&(y=e.bottom-u.bottom+o)):e.bottom>u.bottom-o&&(y=e.bottom-u.bottom+o,t<0&&e.top-y<u.top&&(y=e.top-(u.top+o)));else{let x=e.bottom-e.top,v=u.bottom-u.top;y=(s=="center"&&x<=v?e.top+x/2-v/2:s=="start"||s=="center"&&t<0?e.top-o:e.bottom-v+o)-u.top}if(n=="nearest"?e.left<u.left+r?(g=e.left-(u.left+r),t>0&&e.right>u.right+g&&(g=e.right-u.right+r)):e.right>u.right-r&&(g=e.right-u.right+r,t<0&&e.left<u.left+g&&(g=e.left-(u.left+r))):g=(n=="center"?e.left+(e.right-e.left)/2-(u.right-u.left)/2:n=="start"==a?e.left-r:e.right-(u.right-u.left)+r)-u.left,g||y)if(f)h.scrollBy(g,y);else{let x=0,v=0;if(y){let I=c.scrollTop;c.scrollTop+=y/m,v=(c.scrollTop-I)*m}if(g){let I=c.scrollLeft;c.scrollLeft+=g/p,x=(c.scrollLeft-I)*p}e={left:e.left-x,top:e.top-v,right:e.right-x,bottom:e.bottom-v},x&&Math.abs(x-g)<1&&(n="nearest"),v&&Math.abs(v-y)<1&&(s="nearest")}if(f)break;(e.top<u.top||e.bottom>u.bottom||e.left<u.left||e.right>u.right)&&(e={left:Math.max(e.left,u.left),right:Math.min(e.right,u.right),top:Math.max(e.top,u.top),bottom:Math.min(e.bottom,u.bottom)}),c=c.assignedSlot||c.parentNode}else if(c.nodeType==11)c=c.host;else break}function Nl(i,e=!0){let t=i.ownerDocument,n=null,s=null;for(let r=i.parentNode;r&&!(r==t.body||(!e||n)&&s);)if(r.nodeType==1)!s&&r.scrollHeight>r.clientHeight&&(s=r),e&&!n&&r.scrollWidth>r.clientWidth&&(n=r),r=r.assignedSlot||r.parentNode;else if(r.nodeType==11)r=r.host;else break;return{x:n,y:s}}class Yd{constructor(){this.anchorNode=null,this.anchorOffset=0,this.focusNode=null,this.focusOffset=0}eq(e){return this.anchorNode==e.anchorNode&&this.anchorOffset==e.anchorOffset&&this.focusNode==e.focusNode&&this.focusOffset==e.focusOffset}setRange(e){let{anchorNode:t,focusNode:n}=e;this.set(t,Math.min(e.anchorOffset,t?mt(t):0),n,Math.min(e.focusOffset,n?mt(n):0))}set(e,t,n,s){this.anchorNode=e,this.anchorOffset=t,this.focusNode=n,this.focusOffset=s}}function Wl(i){let e=[];for(let t=i;t;t=t.nodeType==11?t.host:t.parentNode)t.nodeType==1&&e.push({node:t,left:t.scrollLeft,top:t.scrollTop});return e}function ql(i,e=!0){for(let{node:t,left:n,top:s}of i)e&&t.scrollTop!=s&&(t.scrollTop=s),t.scrollLeft!=n&&(t.scrollLeft=n)}let Bt=null;O.safari&&O.safari_version>=26&&(Bt=!1);function Hl(i){if(i.setActive)return i.setActive();if(Bt)return i.focus(Bt);let e=Wl(i);i.focus(Bt==null?{get preventScroll(){return Bt={preventScroll:!0},!0}}:void 0),Bt||(Bt=!1,ql(e))}let Jo;function jn(i,e,t=e){let n=Jo||(Jo=document.createRange());return n.setEnd(i,t),n.setStart(i,e),n}function en(i,e,t,n){let s={key:e,code:e,keyCode:t,which:t,cancelable:!0};n&&({altKey:s.altKey,ctrlKey:s.ctrlKey,shiftKey:s.shiftKey,metaKey:s.metaKey}=n);let r=new KeyboardEvent("keydown",s);r.synthetic=!0,i.dispatchEvent(r);let o=new KeyboardEvent("keyup",s);return o.synthetic=!0,i.dispatchEvent(o),r.defaultPrevented||o.defaultPrevented}function Jd(i){for(;i;){if(i&&(i.nodeType==9||i.nodeType==11&&i.host))return i;i=i.assignedSlot||i.parentNode}return null}function Xd(i,e){let t=e.focusNode,n=e.focusOffset;if(!t||e.anchorNode!=t||e.anchorOffset!=n)return!1;for(n=Math.min(n,mt(t));;)if(n){if(t.nodeType!=1)return!1;let s=t.childNodes[n-1];s.contentEditable=="false"?n--:(t=s,n=mt(t))}else{if(t==i)return!0;n=It(t),t=t.parentNode}}function Fl(i){return i instanceof Window?i.pageYOffset>Math.max(0,i.document.documentElement.scrollHeight-i.innerHeight-4):i.scrollTop>Math.max(1,i.scrollHeight-i.clientHeight-4)}function Vl(i,e){for(let t=i,n=e;;){if(t.nodeType==3&&n>0)return{node:t,offset:n};if(t.nodeType==1&&n>0){if(t.contentEditable=="false")return null;t=t.childNodes[n-1],n=mt(t)}else if(t.parentNode&&!qi(t))n=It(t),t=t.parentNode;else return null}}function zl(i,e){for(let t=i,n=e;;){if(t.nodeType==3&&n<t.nodeValue.length)return{node:t,offset:n};if(t.nodeType==1&&n<t.childNodes.length){if(t.contentEditable=="false")return null;t=t.childNodes[n],n=0}else if(t.parentNode&&!qi(t))n=It(t)+1,t=t.parentNode;else return null}}class Ge{constructor(e,t,n=!0){this.node=e,this.offset=t,this.precise=n}static before(e,t){return new Ge(e.parentNode,It(e),t)}static after(e,t){return new Ge(e.parentNode,It(e)+1,t)}}var J=function(i){return i[i.LTR=0]="LTR",i[i.RTL=1]="RTL",i}(J||(J={}));const zt=J.LTR,to=J.RTL;function jl(i){let e=[];for(let t=0;t<i.length;t++)e.push(1<<+i[t]);return e}const Qd=jl("88888888888888888888888888888888888666888888787833333333337888888000000000000000000000000008888880000000000000000000000000088888888888888888888888888888888888887866668888088888663380888308888800000000000000000000000800000000000000000000000000000008"),Zd=jl("4444448826627288999999999992222222222222222222222222222222222222222222222229999999999999999999994444444444644222822222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222222999999949999999229989999223333333333"),Cr=Object.create(null),et=[];for(let i of["()","[]","{}"]){let e=i.charCodeAt(0),t=i.charCodeAt(1);Cr[e]=t,Cr[t]=-e}function Ul(i){return i<=247?Qd[i]:1424<=i&&i<=1524?2:1536<=i&&i<=1785?Zd[i-1536]:1774<=i&&i<=2220?4:8192<=i&&i<=8204?256:64336<=i&&i<=65023?4:1}const eu=/[\u0590-\u05f4\u0600-\u06ff\u0700-\u08ac\ufb50-\ufdff]/;class at{get dir(){return this.level%2?to:zt}constructor(e,t,n){this.from=e,this.to=t,this.level=n}side(e,t){return this.dir==t==e?this.to:this.from}forward(e,t){return e==(this.dir==t)}static find(e,t,n,s){let r=-1;for(let o=0;o<e.length;o++){let a=e[o];if(a.from<=t&&a.to>=t){if(a.level==n)return o;(r<0||(s!=0?s<0?a.from<t:a.to>t:e[r].level>a.level))&&(r=o)}}if(r<0)throw new RangeError("Index out of range");return r}}function $l(i,e){if(i.length!=e.length)return!1;for(let t=0;t<i.length;t++){let n=i[t],s=e[t];if(n.from!=s.from||n.to!=s.to||n.direction!=s.direction||!$l(n.inner,s.inner))return!1}return!0}const Y=[];function tu(i,e,t,n,s){for(let r=0;r<=n.length;r++){let o=r?n[r-1].to:e,a=r<n.length?n[r].from:t,l=r?256:s;for(let h=o,c=l,d=l;h<a;h++){let u=Ul(i.charCodeAt(h));u==512?u=c:u==8&&d==4&&(u=16),Y[h]=u==4?2:u,u&7&&(d=u),c=u}for(let h=o,c=l,d=l;h<a;h++){let u=Y[h];if(u==128)h<a-1&&c==Y[h+1]&&c&24?u=Y[h]=c:Y[h]=256;else if(u==64){let f=h+1;for(;f<a&&Y[f]==64;)f++;let p=h&&c==8||f<t&&Y[f]==8?d==1?1:8:256;for(let m=h;m<f;m++)Y[m]=p;h=f-1}else u==8&&d==1&&(Y[h]=1);c=u,u&7&&(d=u)}}}function nu(i,e,t,n,s){let r=s==1?2:1;for(let o=0,a=0,l=0;o<=n.length;o++){let h=o?n[o-1].to:e,c=o<n.length?n[o].from:t;for(let d=h,u,f,p;d<c;d++)if(f=Cr[u=i.charCodeAt(d)])if(f<0){for(let m=a-3;m>=0;m-=3)if(et[m+1]==-f){let g=et[m+2],y=g&2?s:g&4?g&1?r:s:0;y&&(Y[d]=Y[et[m]]=y),a=m;break}}else{if(et.length==189)break;et[a++]=d,et[a++]=u,et[a++]=l}else if((p=Y[d])==2||p==1){let m=p==s;l=m?0:1;for(let g=a-3;g>=0;g-=3){let y=et[g+2];if(y&2)break;if(m)et[g+2]|=2;else{if(y&4)break;et[g+2]|=4}}}}}function iu(i,e,t,n){for(let s=0,r=n;s<=t.length;s++){let o=s?t[s-1].to:i,a=s<t.length?t[s].from:e;for(let l=o;l<a;){let h=Y[l];if(h==256){let c=l+1;for(;;)if(c==a){if(s==t.length)break;c=t[s++].to,a=s<t.length?t[s].from:e}else if(Y[c]==256)c++;else break;let d=r==1,u=(c<e?Y[c]:n)==1,f=d==u?d?1:2:n;for(let p=c,m=s,g=m?t[m-1].to:i;p>l;)p==g&&(p=t[--m].from,g=m?t[m-1].to:i),Y[--p]=f;l=c}else r=h,l++}}}function Ar(i,e,t,n,s,r,o){let a=n%2?2:1;if(n%2==s%2)for(let l=e,h=0;l<t;){let c=!0,d=!1;if(h==r.length||l<r[h].from){let m=Y[l];m!=a&&(c=!1,d=m==16)}let u=!c&&a==1?[]:null,f=c?n:n+1,p=l;e:for(;;)if(h<r.length&&p==r[h].from){if(d)break e;let m=r[h];if(!c)for(let g=m.to,y=h+1;;){if(g==t)break e;if(y<r.length&&r[y].from==g)g=r[y++].to;else{if(Y[g]==a)break e;break}}if(h++,u)u.push(m);else{m.from>l&&o.push(new at(l,m.from,f));let g=m.direction==zt!=!(f%2);Tr(i,g?n+1:n,s,m.inner,m.from,m.to,o),l=m.to}p=m.to}else{if(p==t||(c?Y[p]!=a:Y[p]==a))break;p++}u?Ar(i,l,p,n+1,s,u,o):l<p&&o.push(new at(l,p,f)),l=p}else for(let l=t,h=r.length;l>e;){let c=!0,d=!1;if(!h||l>r[h-1].to){let m=Y[l-1];m!=a&&(c=!1,d=m==16)}let u=!c&&a==1?[]:null,f=c?n:n+1,p=l;e:for(;;)if(h&&p==r[h-1].to){if(d)break e;let m=r[--h];if(!c)for(let g=m.from,y=h;;){if(g==e)break e;if(y&&r[y-1].to==g)g=r[--y].from;else{if(Y[g-1]==a)break e;break}}if(u)u.push(m);else{m.to<l&&o.push(new at(m.to,l,f));let g=m.direction==zt!=!(f%2);Tr(i,g?n+1:n,s,m.inner,m.from,m.to,o),l=m.from}p=m.from}else{if(p==e||(c?Y[p-1]!=a:Y[p-1]==a))break;p--}u?Ar(i,p,l,n+1,s,u,o):p<l&&o.push(new at(p,l,f)),l=p}}function Tr(i,e,t,n,s,r,o){let a=e%2?2:1;tu(i,s,r,n,a),nu(i,s,r,n,a),iu(s,r,n,a),Ar(i,s,r,e,t,n,o)}function su(i,e,t){if(!i)return[new at(0,0,e==to?1:0)];if(e==zt&&!t.length&&!eu.test(i))return Kl(i.length);if(t.length)for(;i.length>Y.length;)Y[Y.length]=256;let n=[],s=e==zt?0:1;return Tr(i,s,s,t,0,i.length,n),n}function Kl(i){return[new at(0,i,0)]}let Gl="";function ru(i,e,t,n,s){var r;if(!i.length)return null;let o=n.head-i.from,a;if(n.head==i.from&&n.assoc<0){if(!s)return null;o=e[a=0].side(!1,t)}else if(n.head==i.to&&n.assoc>0){if(s)return null;o=e[a=e.length-1].side(!0,t)}else a=at.find(e,o,(r=n.bidiLevel)!==null&&r!==void 0?r:-1,n.assoc);let l=e[a],h=l.side(s,t);if(o==h){let u=a+=s?1:-1;if(u<0||u>=e.length)return null;l=e[a=u],o=l.side(!s,t),h=l.side(s,t)}let c=he(i.text,o,l.forward(s,t));(c<l.from||c>l.to)&&(c=h),Gl=i.text.slice(Math.min(o,c),Math.max(o,c));let d=a==(s?e.length-1:0)?null:e[a+(s?1:-1)];if(c==h){if(!d)return s?b.cursor(i.to,1):b.cursor(i.from,-1);if(d.level+(s?0:1)<l.level)return b.cursor(d.side(!s,t)+i.from,d.forward(s,t)?1:-1,d.level)}return b.cursor(c+i.from,l.forward(s,t)?-1:1,l.level)}function ou(i,e,t){for(let n=e;n<t;n++){let s=Ul(i.charCodeAt(n));if(s==1)return zt;if(s==2||s==4)return to}return zt}const Yl=D.define(),Jl=D.define(),Xl=D.define(),Ql=D.define(),Mr=D.define(),Zl=D.define(),eh=D.define(),no=D.define(),io=D.define(),th=D.define({combine:i=>i.some(e=>e)}),nh=D.define({combine:i=>i.some(e=>e)}),ih=D.define();class tn{constructor(e,t,n,s,r,o=!1){this.range=e,this.y=t,this.x=n,this.yMargin=s,this.xMargin=r,this.isSnapshot=o}map(e){return e.empty?this:new tn(this.range.map(e),this.y,this.x,this.yMargin,this.xMargin,this.isSnapshot)}clip(e){return this.range.to<=e.doc.length?this:new tn(b.cursor(e.doc.length),this.y,this.x,this.yMargin,this.xMargin,this.isSnapshot)}}const oi=N.define({map:(i,e)=>i.map(e)}),sh=N.define();function Ce(i,e,t){let n=i.facet(Ql);n.length?n[0](e):window.onerror&&window.onerror(String(e),t,void 0,void 0,e)||(t?console.error(t+":",e):console.error(e))}const ut=D.define({combine:i=>i.length?i[0]:!0});let au=0;const Yt=D.define({combine(i){return i.filter((e,t)=>{for(let n=0;n<t;n++)if(i[n].plugin==e.plugin)return!1;return!0})}});class ce{constructor(e,t,n,s,r){this.id=e,this.create=t,this.domEventHandlers=n,this.domEventObservers=s,this.baseExtensions=r(this),this.extension=this.baseExtensions.concat(Yt.of({plugin:this,arg:void 0}))}of(e){return this.baseExtensions.concat(Yt.of({plugin:this,arg:e}))}static define(e,t){const{eventHandlers:n,eventObservers:s,provide:r,decorations:o}=t||{};return new ce(au++,e,n,s,a=>{let l=[];return o&&l.push(ds.of(h=>{let c=h.plugin(a);return c?o(c):_.none})),r&&l.push(r(a)),l})}static fromClass(e,t){return ce.define((n,s)=>new e(n,s),t)}}class Ns{constructor(e){this.spec=e,this.mustUpdate=null,this.value=null}get plugin(){return this.spec&&this.spec.plugin}update(e){if(this.value){if(this.mustUpdate){let t=this.mustUpdate;if(this.mustUpdate=null,this.value.update)try{this.value.update(t)}catch(n){if(Ce(t.state,n,"CodeMirror plugin crashed"),this.value.destroy)try{this.value.destroy()}catch{}this.deactivate()}}}else if(this.spec)try{this.value=this.spec.plugin.create(e,this.spec.arg)}catch(t){Ce(e.state,t,"CodeMirror plugin crashed"),this.deactivate()}return this}destroy(e){var t;if(!((t=this.value)===null||t===void 0)&&t.destroy)try{this.value.destroy()}catch(n){Ce(e.state,n,"CodeMirror plugin crashed")}}deactivate(){this.spec=this.value=null}}const rh=D.define(),so=D.define(),ds=D.define(),oh=D.define(),ro=D.define(),ei=D.define(),ah=D.define();function Xo(i,e){let t=i.state.facet(ah);if(!t.length)return t;let n=t.map(r=>r instanceof Function?r(i):r),s=[];return H.spans(n,e.from,e.to,{point(){},span(r,o,a,l){let h=r-e.from,c=o-e.from,d=s;for(let u=a.length-1;u>=0;u--,l--){let f=a[u].spec.bidiIsolate,p;if(f==null&&(f=ou(e.text,h,c)),l>0&&d.length&&(p=d[d.length-1]).to==h&&p.direction==f)p.to=c,d=p.inner;else{let m={from:h,to:c,direction:f,inner:[]};d.push(m),d=m.inner}}}}),s}const lh=D.define();function oo(i){let e=0,t=0,n=0,s=0;for(let r of i.state.facet(lh)){let o=r(i);o&&(o.left!=null&&(e=Math.max(e,o.left)),o.right!=null&&(t=Math.max(t,o.right)),o.top!=null&&(n=Math.max(n,o.top)),o.bottom!=null&&(s=Math.max(s,o.bottom)))}return{left:e,right:t,top:n,bottom:s}}const On=D.define();class qe{constructor(e,t,n,s){this.fromA=e,this.toA=t,this.fromB=n,this.toB=s}join(e){return new qe(Math.min(this.fromA,e.fromA),Math.max(this.toA,e.toA),Math.min(this.fromB,e.fromB),Math.max(this.toB,e.toB))}addToSet(e){let t=e.length,n=this;for(;t>0;t--){let s=e[t-1];if(!(s.fromA>n.toA)){if(s.toA<n.fromA)break;n=n.join(s),e.splice(t-1,1)}}return e.splice(t,0,n),e}static extendWithRanges(e,t){if(t.length==0)return e;let n=[];for(let s=0,r=0,o=0;;){let a=s<e.length?e[s].fromB:1e9,l=r<t.length?t[r]:1e9,h=Math.min(a,l);if(h==1e9)break;let c=h+o,d=h,u=c;for(;;)if(r<t.length&&t[r]<=d){let f=t[r+1];r+=2,d=Math.max(d,f);for(let p=s;p<e.length&&e[p].fromB<=d;p++)o=e[p].toA-e[p].toB;u=Math.max(u,f+o)}else if(s<e.length&&e[s].fromB<=d){let f=e[s++];d=Math.max(d,f.toB),u=Math.max(u,f.toA),o=f.toA-f.toB}else break;n.push(new qe(c,u,h,d))}return n}}class Fi{constructor(e,t,n){this.view=e,this.state=t,this.transactions=n,this.flags=0,this.startState=e.state,this.changes=oe.empty(this.startState.doc.length);for(let r of n)this.changes=this.changes.compose(r.changes);let s=[];this.changes.iterChangedRanges((r,o,a,l)=>s.push(new qe(r,o,a,l))),this.changedRanges=s}static create(e,t,n){return new Fi(e,t,n)}get viewportChanged(){return(this.flags&4)>0}get viewportMoved(){return(this.flags&8)>0}get heightChanged(){return(this.flags&2)>0}get geometryChanged(){return this.docChanged||(this.flags&18)>0}get focusChanged(){return(this.flags&1)>0}get docChanged(){return!this.changes.empty}get selectionSet(){return this.transactions.some(e=>e.selection)}get empty(){return this.flags==0&&this.transactions.length==0}}const lu=[];class te{constructor(e,t,n=0){this.dom=e,this.length=t,this.flags=n,this.parent=null,e.cmTile=this}get breakAfter(){return this.flags&1}get children(){return lu}isWidget(){return!1}get isHidden(){return!1}isComposite(){return!1}isLine(){return!1}isText(){return!1}isBlock(){return!1}get domAttrs(){return null}sync(e){if(this.flags|=2,this.flags&4){this.flags&=-5;let t=this.domAttrs;t&&jd(this.dom,t)}}toString(){return this.constructor.name+(this.children.length?`(${this.children})`:"")+(this.breakAfter?"#":"")}destroy(){this.parent=null}setDOM(e){this.dom=e,e.cmTile=this}get posAtStart(){return this.parent?this.parent.posBefore(this):0}get posAtEnd(){return this.posAtStart+this.length}posBefore(e,t=this.posAtStart){let n=t;for(let s of this.children){if(s==e)return n;n+=s.length+s.breakAfter}throw new RangeError("Invalid child in posBefore")}posAfter(e){return this.posBefore(e)+e.length}covers(e){return!0}coordsIn(e,t,n){return null}domPosFor(e,t){let n=It(this.dom),s=this.length?e>0:t>0;return new Ge(this.parent.dom,n+(s?1:0),e==0||e==this.length)}markDirty(e){this.flags&=-3,e&&(this.flags|=4),this.parent&&this.parent.flags&2&&this.parent.markDirty(!1)}get overrideDOMText(){return null}get root(){for(let e=this;e;e=e.parent)if(e instanceof fs)return e;return null}static get(e){return e.cmTile}}class us extends te{constructor(e){super(e,0),this._children=[]}isComposite(){return!0}get children(){return this._children}get lastChild(){return this.children.length?this.children[this.children.length-1]:null}append(e){this.children.push(e),e.parent=this}sync(e){if(this.flags&2)return;super.sync(e);let t=this.dom,n=null,s,r=(e==null?void 0:e.node)==t?e:null,o=0;for(let a of this.children){if(a.sync(e),o+=a.length+a.breakAfter,s=n?n.nextSibling:t.firstChild,r&&s!=a.dom&&(r.written=!0),a.dom.parentNode==t)for(;s&&s!=a.dom;)s=Qo(s);else t.insertBefore(a.dom,s);n=a.dom}for(s=n?n.nextSibling:t.firstChild,r&&s&&(r.written=!0);s;)s=Qo(s);this.length=o}}function Qo(i){let e=i.nextSibling;return i.parentNode.removeChild(i),e}class fs extends us{constructor(e,t){super(t),this.view=e}owns(e){for(;e;e=e.parent)if(e==this)return!0;return!1}isBlock(){return!0}nearest(e){for(;;){if(!e)return null;let t=te.get(e);if(t&&this.owns(t))return t;e=e.parentNode}}blockTiles(e){for(let t=[],n=this,s=0,r=0;;)if(s==n.children.length){if(!t.length)return;n=n.parent,n.breakAfter&&r++,s=t.pop()}else{let o=n.children[s++];if(o instanceof pt)t.push(s),n=o,s=0;else{let a=r+o.length,l=e(o,r);if(l!==void 0)return l;r=a+o.breakAfter}}}resolveBlock(e,t){let n,s=-1,r,o=-1;if(this.blockTiles((a,l)=>{let h=l+a.length;if(e>=l&&e<=h){if(a.isWidget()&&t>=-1&&t<=1){if(a.flags&32)return!0;a.flags&16&&(n=void 0)}(l<e||e==h&&(t<-1?a.length:a.covers(1)))&&(!n||!a.isWidget()&&n.isWidget())&&(n=a,s=e-l),(h>e||e==l&&(t>1?a.length:a.covers(-1)))&&(!r||!a.isWidget()&&r.isWidget())&&(r=a,o=e-l)}}),!n&&!r)throw new Error("No tile at position "+e);return n&&t<0||!r?{tile:n,offset:s}:{tile:r,offset:o}}}class pt extends us{constructor(e,t){super(e),this.wrapper=t}isBlock(){return!0}covers(e){return this.children.length?e<0?this.children[0].covers(-1):this.lastChild.covers(1):!1}get domAttrs(){return this.wrapper.attributes}static of(e,t){let n=new pt(t||document.createElement(e.tagName),e);return t||(n.flags|=4),n}}class an extends us{constructor(e,t){super(e),this.attrs=t}isLine(){return!0}static start(e,t,n){let s=new an(t||document.createElement("div"),e);return(!t||!n)&&(s.flags|=4),s}get domAttrs(){return this.attrs}resolveInline(e,t,n){let s=null,r=-1,o=null,a=-1;function l(c,d){for(let u=0,f=0;u<c.children.length&&f<=d;u++){let p=c.children[u],m=f+p.length;m>=d&&(p.isComposite()?l(p,d-f):(!o||o.isHidden&&(t>0&&!(o.flags&32)||n&&cu(o,p)))&&(m>d||p.flags&32&&t<=1)?(o=p,a=d-f):(f<d||p.flags&16&&!p.isHidden&&t>=-1)&&(s=p,r=d-f)),f=m}}l(this,e);let h=(t<0?s:o)||s||o;return h?{tile:h,offset:h==s?r:a}:null}coordsIn(e,t,n){let s=this.resolveInline(e,t,!0);return s?s.tile.coordsIn(Math.max(0,s.offset),t,n):hu(this)}domIn(e,t){let n=this.resolveInline(e,t);if(n){let{tile:s,offset:r}=n;if(this.dom.contains(s.dom))return s.isText()?new Ge(s.dom,Math.min(s.dom.nodeValue.length,r)):s.domPosFor(r,s.flags&16?1:s.flags&32?-1:t);let o=n.tile.parent,a=!1;for(let l of o.children){if(a)return new Ge(l.dom,0);l==n.tile&&(a=!0)}}return new Ge(this.dom,0)}}function hu(i){let e=i.dom.lastChild;if(!e)return i.dom.getBoundingClientRect();let t=Ii(e);return t[t.length-1]||null}function cu(i,e){let t=i.coordsIn(0,1),n=e.coordsIn(0,1);return t&&n&&n.top<t.bottom}class De extends us{constructor(e,t){super(e),this.mark=t}get domAttrs(){return this.mark.attrs}static of(e,t){let n=new De(t||document.createElement(e.tagName),e);return t||(n.flags|=4),n}}class Nt extends te{constructor(e,t){super(e,t.length),this.text=t}sync(e){this.flags&2||(super.sync(e),this.dom.nodeValue!=this.text&&(e&&e.node==this.dom&&(e.written=!0),this.dom.nodeValue=this.text))}isText(){return!0}toString(){return JSON.stringify(this.text)}coordsIn(e,t,n){let s=this.dom.nodeValue.length;e>s&&(e=s);let r=e,o=e,a=0;e==0&&t<0||e==s&&t>=0?O.chrome||O.gecko||(e?(r--,a=1):o<s&&(o++,a=-1)):t<0?r--:o<s&&o++;let l=jn(this.dom,r,o).getClientRects();if(!l.length)return null;let h=l[(a?a<0:t>=0)?0:l.length-1];return O.safari&&!a&&h.width==0&&(h=Array.prototype.find.call(l,c=>c.width)||h),n==null?h:Hi(h,(a?a>0:t<0)==n)}static of(e,t){let n=new Nt(t||document.createTextNode(e),e);return t||(n.flags|=2),n}}class jt extends te{constructor(e,t,n,s){super(e,t,s),this.widget=n}isWidget(){return!0}get isHidden(){return this.widget.isHidden}covers(e){return this.flags&48?!1:(this.flags&(e<0?64:128))>0}coordsIn(e,t){return this.coordsInWidget(e,t,!1)}coordsInWidget(e,t,n){let s=this.widget.coordsAt(this.dom,e,t);if(s)return s;if(n)return Hi(this.dom.getBoundingClientRect(),this.length?e==0:t<=0);{let r=this.dom.getClientRects(),o=null;if(!r.length)return null;let a=this.flags&16?!0:this.flags&32?!1:e>0;for(let l=a?r.length-1:0;o=r[l],!(e>0?l==0:l==r.length-1||o.top<o.bottom);l+=a?-1:1);return Hi(o,!a)}}get overrideDOMText(){if(!this.length)return j.empty;let{root:e}=this;if(!e)return j.empty;let t=this.posAtStart;return e.view.state.doc.slice(t,t+this.length)}destroy(){super.destroy(),this.widget.destroy(this.dom)}static of(e,t,n,s,r){return r||(r=e.toDOM(t),e.editable||(r.contentEditable="false")),new jt(r,n,e,s)}}class Vi extends te{constructor(e){let t=document.createElement("img");t.className="cm-widgetBuffer",t.setAttribute("aria-hidden","true"),super(t,0,e)}get isHidden(){return!0}get overrideDOMText(){return j.empty}coordsIn(e,t,n){let s=this.dom.getBoundingClientRect();return n==null?s:Hi(s,t>0==n)}}class du{constructor(e){this.index=0,this.beforeBreak=!1,this.parents=[],this.tile=e}advance(e,t,n){let{tile:s,index:r,beforeBreak:o,parents:a}=this;for(;e||t>0;)if(s.isComposite())if(o){if(!e)break;n&&n.break(),e--,o=!1}else if(r==s.children.length){if(!e&&!a.length)break;n&&n.leave(s),o=!!s.breakAfter,{tile:s,index:r}=a.pop(),r++}else{let l=s.children[r],h=l.breakAfter;(t>0?l.length<=e:l.length<e)&&(!n||n.skip(l,0,l.length)!==!1||!l.isComposite)?(o=!!h,r++,e-=l.length):(a.push({tile:s,index:r}),s=l,r=0,n&&l.isComposite()&&n.enter(l))}else{let l=s.length;if(r<l&&e){let h=Math.min(e,l-r);n&&n.skip(s,r,r+h),e-=h,r+=h}if(r==l)o=!!s.breakAfter,{tile:s,index:r}=a.pop(),r++;else if(!e)break}return this.tile=s,this.index=r,this.beforeBreak=o,this}get root(){return this.parents.length?this.parents[0].tile:this.tile}}class uu{constructor(e,t,n,s){this.from=e,this.to=t,this.wrapper=n,this.rank=s}}class fu{constructor(e,t,n){this.cache=e,this.root=t,this.blockWrappers=n,this.curLine=null,this.lastBlock=null,this.afterWidget=null,this.pos=0,this.wrappers=[],this.wrapperPos=0}addText(e,t,n,s){var r;this.flushBuffer();let o=this.ensureMarks(t,n),a=o.lastChild;if(a&&a.isText()&&!(a.flags&8)&&a.length+e.length<512){this.cache.reused.set(a,2);let l=o.children[o.children.length-1]=new Nt(a.dom,a.text+e);l.parent=o}else o.append(s||Nt.of(e,(r=this.cache.find(Nt))===null||r===void 0?void 0:r.dom));this.pos+=e.length,this.afterWidget=null}addComposition(e,t){let n=this.curLine;n.dom!=t.line.dom&&(n.setDOM(this.cache.reused.has(t.line)?Ws(t.line.dom):t.line.dom),this.cache.reused.set(t.line,2));let s=n;for(let a=t.marks.length-1;a>=0;a--){let l=t.marks[a],h=s.lastChild;if(h instanceof De&&h.mark.eq(l.mark))h.dom!=l.dom&&h.setDOM(Ws(l.dom)),s=h;else{let{dom:c}=l;this.cache.reused.get(l)&&te.get(l.dom)&&(c=Ws(l.dom));let d=De.of(l.mark,c);s.append(d),s=d}this.cache.reused.set(l,2)}let r=te.get(e.text);r&&this.cache.reused.set(r,2);let o=new Nt(e.text,e.text.nodeValue);o.flags|=8,this.pos=e.range.toB,s.append(o)}addInlineWidget(e,t,n){let s=this.afterWidget&&e.flags&48&&(this.afterWidget.flags&48)==(e.flags&48);s||this.flushBuffer();let r=this.ensureMarks(t,n);!s&&!(e.flags&16)&&r.append(this.getBuffer(1)),r.append(e),this.pos+=e.length,this.afterWidget=e}addMark(e,t,n){this.flushBuffer(),this.ensureMarks(t,n).append(e),this.pos+=e.length,this.afterWidget=null}addBlockWidget(e){this.getBlockPos().append(e),this.pos+=e.length,this.lastBlock=e,this.endLine()}continueWidget(e){let t=this.afterWidget||this.lastBlock;t.length+=e,this.pos+=e}addLineStart(e,t){var n;e||(e=hh);let s=an.start(e,t||((n=this.cache.find(an))===null||n===void 0?void 0:n.dom),!!t);this.getBlockPos().append(this.lastBlock=this.curLine=s)}addLine(e){this.getBlockPos().append(e),this.pos+=e.length,this.lastBlock=e,this.endLine()}addBreak(){this.lastBlock.flags|=1,this.endLine(),this.pos++}addLineStartIfNotCovered(e){this.blockPosCovered()||this.addLineStart(e)}ensureLine(e){this.curLine||this.addLineStart(e)}ensureMarks(e,t){var n;let s=this.curLine;for(let r=e.length-1;r>=0;r--){let o=e[r],a;if(t>0&&(a=s.lastChild)&&a instanceof De&&a.mark.eq(o))s=a,t--;else{let l=De.of(o,(n=this.cache.find(De,h=>h.mark.eq(o)))===null||n===void 0?void 0:n.dom);s.append(l),s=l,t=0}}return s}endLine(){if(this.curLine){this.flushBuffer();let e=this.curLine.lastChild;(!e||!Zo(this.curLine,!1)||e.dom.nodeName!="BR"&&e.isWidget()&&!(O.ios&&Zo(this.curLine,!0)))&&this.curLine.append(this.cache.findWidget(qs,0,32)||new jt(qs.toDOM(),0,qs,32)),this.curLine=this.afterWidget=null}}updateBlockWrappers(){this.wrapperPos>this.pos+1e4&&(this.blockWrappers.goto(this.pos),this.wrappers.length=0);for(let e=this.wrappers.length-1;e>=0;e--)this.wrappers[e].to<this.pos&&this.wrappers.splice(e,1);for(let e=this.blockWrappers;e.value&&e.from<=this.pos;e.next())if(e.to>=this.pos){let t=e.rank*102+e.value.rank,n=new uu(e.from,e.to,e.value,t),s=this.wrappers.length;for(;s>0&&(this.wrappers[s-1].rank-n.rank||this.wrappers[s-1].to-n.to)<0;)s--;this.wrappers.splice(s,0,n)}this.wrapperPos=this.pos}getBlockPos(){var e;this.updateBlockWrappers();let t=this.root;for(let n of this.wrappers){let s=t.lastChild;if(n.from<this.pos&&s instanceof pt&&s.wrapper.eq(n.wrapper))t=s;else{let r=pt.of(n.wrapper,(e=this.cache.find(pt,o=>o.wrapper.eq(n.wrapper)))===null||e===void 0?void 0:e.dom);t.append(r),t=r}}return t}blockPosCovered(){let e=this.lastBlock;return e!=null&&!e.breakAfter&&(!e.isWidget()||(e.flags&160)>0)}getBuffer(e){let t=2|(e<0?16:32),n=this.cache.find(Vi,void 0,1);return n&&(n.flags=t),n||new Vi(t)}flushBuffer(){this.afterWidget&&!(this.afterWidget.flags&32)&&(this.afterWidget.parent.append(this.getBuffer(-1)),this.afterWidget=null)}}class pu{constructor(e){this.skipCount=0,this.text="",this.textOff=0,this.cursor=e.iter()}skip(e){this.textOff+e<=this.text.length?this.textOff+=e:(this.skipCount+=e-(this.text.length-this.textOff),this.text="",this.textOff=0)}next(e){if(this.textOff==this.text.length){let{value:s,lineBreak:r,done:o}=this.cursor.next(this.skipCount);if(this.skipCount=0,o)throw new Error("Ran out of text content when drawing inline views");this.text=s;let a=this.textOff=Math.min(e,s.length);return r?null:s.slice(0,a)}let t=Math.min(this.text.length,this.textOff+e),n=this.text.slice(this.textOff,t);return this.textOff=t,n}}const zi=[jt,an,Nt,De,Vi,pt,fs];for(let i=0;i<zi.length;i++)zi[i].bucket=i;class mu{constructor(e){this.view=e,this.buckets=zi.map(()=>[]),this.index=zi.map(()=>0),this.reused=new Map}add(e){let t=e.constructor.bucket,n=this.buckets[t];n.length<6?n.push(e):n[this.index[t]=(this.index[t]+1)%6]=e}find(e,t,n=2){let s=e.bucket,r=this.buckets[s],o=this.index[s];for(let a=0;a<r.length;a++){let l=(a+o)%r.length,h=r[l];if((!t||t(h))&&!this.reused.has(h))return r.splice(l,1),l<o&&this.index[s]--,this.reused.set(h,n),h}return null}findWidget(e,t,n){let s=this.buckets[0];if(s.length)for(let r=0,o=0;;r++){if(r==s.length){if(o)return null;o=1,r=0}let a=s[r];if(!this.reused.has(a)&&(o==0?a.widget.compare(e):a.widget.constructor==e.constructor&&e.updateDOM(a.dom,this.view,a.widget)))return s.splice(r,1),r<this.index[0]&&this.index[0]--,a.widget==e&&a.length==t&&(a.flags&497)==n?(this.reused.set(a,1),a):(this.reused.set(a,2),new jt(a.dom,t,e,a.flags&-498|n))}}reuse(e){return this.reused.set(e,1),e}maybeReuse(e,t=2){if(!this.reused.has(e))return this.reused.set(e,t),e.dom}clear(){for(let e=0;e<this.buckets.length;e++)this.buckets[e].length=this.index[e]=0}}class gu{constructor(e,t,n,s,r){this.view=e,this.decorations=s,this.disallowBlockEffectsFor=r,this.openWidget=!1,this.openMarks=0,this.cache=new mu(e),this.text=new pu(e.state.doc),this.builder=new fu(this.cache,new fs(e,e.contentDOM),H.iter(n)),this.cache.reused.set(t,2),this.old=new du(t),this.reuseWalker={skip:(o,a,l)=>{if(this.cache.add(o),o.isComposite())return!1},enter:o=>this.cache.add(o),leave:()=>{},break:()=>{}}}run(e,t){let n=t&&this.getCompositionContext(t.text);for(let s=0,r=0,o=0;;){let a=o<e.length?e[o++]:null,l=a?a.fromA:this.old.root.length;if(l>s){let h=l-s;this.preserve(h,!o,!a),s=l,r+=h}if(!a)break;t&&a.fromA<=t.range.fromA&&a.toA>=t.range.toA?(this.forward(a.fromA,t.range.fromA,t.range.fromA<t.range.toA?1:-1),this.emit(r,t.range.fromB),this.builder.flushBuffer(),this.cache.clear(),this.builder.addComposition(t,n),this.text.skip(t.range.toB-t.range.fromB),this.forward(t.range.fromA,a.toA),this.emit(t.range.toB,a.toB)):(this.forward(a.fromA,a.toA),this.emit(r,a.toB)),r=a.toB,s=a.toA}return this.builder.curLine&&this.builder.endLine(),this.builder.root}preserve(e,t,n){let s=wu(this.old),r=this.openMarks;this.old.advance(e,n?1:-1,{skip:(o,a,l)=>{if(o.isWidget())if(this.openWidget)this.builder.continueWidget(l-a);else{let h=l>0||a<o.length?jt.of(o.widget,this.view,l-a,o.flags&496,this.cache.maybeReuse(o)):this.cache.reuse(o);h.flags&256?(h.flags&=-2,this.builder.addBlockWidget(h)):(this.builder.ensureLine(null),this.builder.addInlineWidget(h,s,r),r=s.length)}else if(o.isText())this.builder.ensureLine(null),!a&&l==o.length&&!this.cache.reused.has(o)?this.builder.addText(o.text,s,r,this.cache.reuse(o)):(this.cache.add(o),this.builder.addText(o.text.slice(a,l),s,r)),r=s.length;else if(o.isLine())o.flags&=-2,this.cache.reused.set(o,1),this.builder.addLine(o);else if(o instanceof Vi)this.cache.add(o);else if(o instanceof De)this.builder.ensureLine(null),this.builder.addMark(o,s,r),this.cache.reused.set(o,1),r=s.length;else return!1;this.openWidget=!1},enter:o=>{o.isLine()?this.builder.addLineStart(o.attrs,this.cache.maybeReuse(o)):(this.cache.add(o),o instanceof De&&s.unshift(o.mark)),this.openWidget=!1},leave:o=>{o.isLine()?s.length&&(s.length=r=0):o instanceof De&&(s.shift(),r=Math.min(r,s.length))},break:()=>{this.builder.addBreak(),this.openWidget=!1}}),this.text.skip(e)}emit(e,t){let n=null,s=this.builder,r=-1,o=H.spans(this.decorations,e,t,{point:(a,l,h,c,d,u)=>{if(h instanceof Vt){if(this.disallowBlockEffectsFor[u]){if(h.block)throw new RangeError("Block decorations may not be specified via plugins");if(l>this.view.state.doc.lineAt(a).to)throw new RangeError("Decorations that replace line breaks may not be specified via plugins")}if(r=c.length,d>c.length)s.continueWidget(l-a);else{let f=h.widget||(h.block?ln.block:ln.inline),p=yu(h),m=this.cache.findWidget(f,l-a,p)||jt.of(f,this.view,l-a,p);h.block?(h.startSide>0&&s.addLineStartIfNotCovered(n),s.addBlockWidget(m)):(s.ensureLine(n),s.addInlineWidget(m,c,d))}n=null}else n=bu(n,h);l>a&&this.text.skip(l-a)},span:(a,l,h,c)=>{for(let d=a;d<l;){let u=this.text.next(Math.min(512,l-d));u==null?(s.addLineStartIfNotCovered(n),s.addBreak(),d++):(s.ensureLine(n),s.addText(u,h,d==a?c:h.length),d+=u.length),n=null}r=h.length}});r>-1&&(this.openWidget=o>r),this.openWidget||s.addLineStartIfNotCovered(n),this.openMarks=o}forward(e,t,n=1){t-e<=10?this.old.advance(t-e,n,this.reuseWalker):(this.old.advance(5,-1,this.reuseWalker),this.old.advance(t-e-10,-1),this.old.advance(5,n,this.reuseWalker))}getCompositionContext(e){let t=[],n=null;for(let s=e.parentNode;;s=s.parentNode){let r=te.get(s);if(s==this.view.contentDOM)break;r instanceof De?t.push(r):r!=null&&r.isLine()?n=r:r instanceof pt||(s.nodeName=="DIV"&&!n?n=new an(s,hh):n||t.push(De.of(new Qn({tagName:s.nodeName.toLowerCase(),attributes:Ud(s)}),s)))}return n?{line:n,marks:t}:null}}function Zo(i,e){let t=n=>{for(let s of n.children)if((e?s.isText():s.length)||t(s))return!0;return!1};return t(i)}function yu(i){let e=i.isReplace?(i.startSide<0?64:0)|(i.endSide>0?128:0):i.startSide>0?32:16;return i.block&&(e|=256),e}const hh={class:"cm-line"};function bu(i,e){let t=e.spec.attributes,n=e.spec.class;return!t&&!n||(i||(i={class:"cm-line"}),t&&Zr(t,i),n&&(i.class+=" "+n)),i}function wu(i){let e=[];for(let t=i.parents.length;t>1;t--){let n=t==i.parents.length?i.tile:i.parents[t].tile;n instanceof De&&e.push(n.mark)}return e}function Ws(i){let e=te.get(i);return e&&e.setDOM(i.cloneNode()),i}class ln extends Xn{constructor(e){super(),this.tag=e}eq(e){return e.tag==this.tag}toDOM(){return document.createElement(this.tag)}updateDOM(e){return e.nodeName.toLowerCase()==this.tag}get isHidden(){return!0}}ln.inline=new ln("span");ln.block=new ln("div");const qs=new class extends Xn{toDOM(){return document.createElement("br")}get isHidden(){return!0}get editable(){return!0}};class ea{constructor(e){this.view=e,this.decorations=[],this.blockWrappers=[],this.dynamicDecorationMap=[!1],this.domChanged=null,this.hasComposition=null,this.editContextFormatting=_.none,this.lastCompositionAfterCursor=!1,this.minWidth=0,this.minWidthFrom=0,this.minWidthTo=0,this.impreciseAnchor=null,this.impreciseHead=null,this.forceSelection=!1,this.lastUpdate=Date.now(),this.updateDeco(),this.tile=new fs(e,e.contentDOM),this.updateInner([new qe(0,0,0,e.state.doc.length)],null)}update(e){var t;let n=e.changedRanges;this.minWidth>0&&n.length&&(n.every(({fromA:c,toA:d})=>d<this.minWidthFrom||c>this.minWidthTo)?(this.minWidthFrom=e.changes.mapPos(this.minWidthFrom,1),this.minWidthTo=e.changes.mapPos(this.minWidthTo,1)):this.minWidth=this.minWidthFrom=this.minWidthTo=0),this.updateEditContextFormatting(e);let s=-1;this.view.inputState.composing>=0&&!this.view.observer.editContext&&(!((t=this.domChanged)===null||t===void 0)&&t.newSel?s=this.domChanged.newSel.head:!Ou(e.changes,this.hasComposition)&&!e.selectionSet&&(s=e.state.selection.main.head));let r=s>-1?ku(this.view,e.changes,s):null;if(this.domChanged=null,this.hasComposition){let{from:c,to:d}=this.hasComposition;n=new qe(c,d,e.changes.mapPos(c,-1),e.changes.mapPos(d,1)).addToSet(n.slice())}this.hasComposition=r?{from:r.range.fromB,to:r.range.toB}:null,(O.ie||O.chrome)&&!r&&e&&e.state.doc.lines!=e.startState.doc.lines&&(this.forceSelection=!0);let o=this.decorations,a=this.blockWrappers;this.updateDeco();let l=Cu(o,this.decorations,e.changes);l.length&&(n=qe.extendWithRanges(n,l));let h=Tu(a,this.blockWrappers,e.changes);return h.length&&(n=qe.extendWithRanges(n,h)),r&&!n.some(c=>c.fromA<=r.range.fromA&&c.toA>=r.range.toA)&&(n=r.range.addToSet(n.slice())),this.tile.flags&2&&n.length==0?!1:(this.updateInner(n,r),e.transactions.length&&(this.lastUpdate=Date.now()),!0)}updateInner(e,t){this.view.viewState.mustMeasureContent=!0;let{observer:n}=this.view;n.ignore(()=>{if(t||e.length){let o=this.tile,a=new gu(this.view,o,this.blockWrappers,this.decorations,this.dynamicDecorationMap);t&&te.get(t.text)&&a.cache.reused.set(te.get(t.text),2),this.tile=a.run(e,t),Or(o,a.cache.reused)}this.tile.dom.style.height=this.view.viewState.contentHeight/this.view.scaleY+"px",this.tile.dom.style.flexBasis=this.minWidth?this.minWidth+"px":"";let r=O.chrome||O.ios?{node:n.selectionRange.focusNode,written:!1}:void 0;this.tile.sync(r),r&&(r.written||n.selectionRange.focusNode!=r.node||!this.tile.dom.contains(r.node))&&(this.forceSelection=!0),this.tile.dom.style.height=""});let s=[];if(this.view.viewport.from||this.view.viewport.to<this.view.state.doc.length)for(let r of this.tile.children)r.isWidget()&&r.widget instanceof Hs&&s.push(r.dom);n.updateGaps(s)}updateEditContextFormatting(e){this.editContextFormatting=this.editContextFormatting.map(e.changes);for(let t of e.transactions)for(let n of t.effects)n.is(sh)&&(this.editContextFormatting=n.value)}updateSelection(e=!1,t=!1){(e||!this.view.observer.selectionRange.focusNode)&&this.view.observer.readSelectionRange();let{dom:n}=this.tile,s=this.view.root.activeElement,r=s==n,o=!r&&!(this.view.state.facet(ut)||n.tabIndex>-1)&&En(n,this.view.observer.selectionRange)&&!(s&&n.contains(s));if(!(r||t||o))return;let a=this.forceSelection;this.forceSelection=!1;let l=this.view.state.selection.main,h,c;if(l.empty?c=h=this.inlineDOMNearPos(l.anchor,l.assoc||1):(c=this.inlineDOMNearPos(l.head,l.head==l.from?1:-1),h=this.inlineDOMNearPos(l.anchor,l.anchor==l.from?1:-1)),O.gecko&&l.empty&&!this.hasComposition&&vu(h)){let u=document.createTextNode("");this.view.observer.ignore(()=>h.node.insertBefore(u,h.node.childNodes[h.offset]||null)),h=c=new Ge(u,0),a=!0}let d=this.view.observer.selectionRange;(a||!d.focusNode||(!Ln(h.node,h.offset,d.anchorNode,d.anchorOffset)||!Ln(c.node,c.offset,d.focusNode,d.focusOffset))&&!this.suppressWidgetCursorChange(d,l))&&(this.view.observer.ignore(()=>{O.android&&O.chrome&&n.contains(d.focusNode)&&Mu(d.focusNode,n)&&(n.blur(),n.focus({preventScroll:!0}));let u=zn(this.view.root);if(u)if(l.empty){if(O.gecko){let f=xu(h.node,h.offset);if(f&&f!=3){let p=(f==1?Vl:zl)(h.node,h.offset);p&&(h=new Ge(p.node,p.offset))}}u.collapse(h.node,h.offset),l.bidiLevel!=null&&u.caretBidiLevel!==void 0&&(u.caretBidiLevel=l.bidiLevel)}else if(u.extend){u.collapse(h.node,h.offset);try{u.extend(c.node,c.offset)}catch{}}else{let f=document.createRange();l.anchor>l.head&&([h,c]=[c,h]),f.setEnd(c.node,c.offset),f.setStart(h.node,h.offset),u.removeAllRanges(),u.addRange(f)}o&&this.view.root.activeElement==n&&(n.blur(),s&&s.focus())}),this.view.observer.setSelectionRange(h,c)),this.impreciseAnchor=h.precise?null:new Ge(d.anchorNode,d.anchorOffset),this.impreciseHead=c.precise?null:new Ge(d.focusNode,d.focusOffset)}suppressWidgetCursorChange(e,t){return this.hasComposition&&t.empty&&Ln(e.focusNode,e.focusOffset,e.anchorNode,e.anchorOffset)&&this.posFromDOM(e.focusNode,e.focusOffset)==t.head}enforceCursorAssoc(){if(this.hasComposition)return;let{view:e}=this,t=e.state.selection.main,n=zn(e.root),{anchorNode:s,anchorOffset:r}=e.observer.selectionRange;if(!n||!t.empty||!t.assoc||!n.modify)return;let o=this.lineAt(t.head,t.assoc);if(!o)return;let a=o.posAtStart;if(t.head==a||t.head==a+o.length)return;let l=this.coordsAt(t.head,-1),h=this.coordsAt(t.head,1);if(!l||!h||l.bottom>h.top)return;let c=this.domAtPos(t.head+t.assoc,t.assoc);n.collapse(c.node,c.offset),n.modify("move",t.assoc<0?"forward":"backward","lineboundary"),e.observer.readSelectionRange();let d=e.observer.selectionRange;e.docView.posFromDOM(d.anchorNode,d.anchorOffset)!=t.from&&n.collapse(s,r)}posFromDOM(e,t){let n=this.tile.nearest(e);if(!n)return this.tile.dom.compareDocumentPosition(e)&2?0:this.view.state.doc.length;let s=n.posAtStart;if(n.isComposite()){let r;if(e==n.dom)r=n.dom.childNodes[t];else{let o=mt(e)==0?0:t==0?-1:1;for(;;){let a=e.parentNode;if(a==n.dom)break;o==0&&a.firstChild!=a.lastChild&&(e==a.firstChild?o=-1:o=1),e=a}o<0?r=e:r=e.nextSibling}if(r==n.dom.firstChild)return s;for(;r&&!te.get(r);)r=r.nextSibling;if(!r)return s+n.length;for(let o=0,a=s;;o++){let l=n.children[o];if(l.dom==r)return a;a+=l.length+l.breakAfter}}else return n.isText()?e==n.dom?s+t:s+(t?n.length:0):s}domAtPos(e,t){let{tile:n,offset:s}=this.tile.resolveBlock(e,t);return n.isWidget()?n.domPosFor(s,t):n.domIn(s,t)}inlineDOMNearPos(e,t){let n,s=-1,r=!1,o,a=-1,l=!1;return this.tile.blockTiles((h,c)=>{if(h.isWidget()){if(h.flags&32&&c>=e)return!0;h.flags&16&&(r=!0)}else{let d=c+h.length;if(c<=e&&(n=h,s=e-c,r=d<e),d>=e&&!o&&(o=h,a=e-c,l=c>e),c>e&&o)return!0}}),!n&&!o?this.domAtPos(e,t):(r&&o?n=null:l&&n&&(o=null),n&&t<0||!o?n.domIn(s,t):o.domIn(a,t))}coordsAt(e,t,n){let{tile:s,offset:r}=this.tile.resolveBlock(e,t);return s.isWidget()?s.widget instanceof Hs?null:s.coordsInWidget(r,t,!0):s.coordsIn(r,t,n)}lineAt(e,t){let{tile:n}=this.tile.resolveBlock(e,t);return n.isLine()?n:null}coordsForChar(e){let{tile:t,offset:n}=this.tile.resolveBlock(e,1);if(!t.isLine())return null;function s(r,o){if(r.isComposite())for(let a of r.children){if(a.length>=o){let l=s(a,o);if(l)return l}if(o-=a.length,o<0)break}else if(r.isText()&&o<r.length){let a=he(r.text,o);if(a==o)return null;let l=jn(r.dom,o,a).getClientRects();for(let h=0;h<l.length;h++){let c=l[h];if(h==l.length-1||c.top<c.bottom&&c.left<c.right)return c}}return null}return s(t,n)}measureVisibleLineHeights(e){let t=[],{from:n,to:s}=e,r=this.view.contentDOM.clientWidth,o=r>Math.max(this.view.scrollDOM.clientWidth,this.minWidth)+1,a=-1,l=this.view.textDirection==J.LTR,h=0,c=(d,u,f)=>{for(let p=0;p<d.children.length&&!(u>s);p++){let m=d.children[p],g=u+m.length,y=m.dom.getBoundingClientRect(),{height:x}=y;if(f&&!p&&(h+=y.top-f.top),m instanceof pt)g>n&&c(m,u,y);else if(u>=n&&(h>0&&t.push(-h),t.push(x+h),h=0,o)){let v=m.dom.lastChild,I=v?Ii(v):[];if(I.length){let S=I[I.length-1],C=l?S.right-y.left:y.right-S.left;C>a&&(a=C,this.minWidth=r,this.minWidthFrom=u,this.minWidthTo=g)}}f&&p==d.children.length-1&&(h+=f.bottom-y.bottom),u=g+m.breakAfter}};return c(this.tile,0,null),t}textDirectionAt(e){let{tile:t}=this.tile.resolveBlock(e,1);return getComputedStyle(t.dom).direction=="rtl"?J.RTL:J.LTR}measureTextSize(){let e=this.tile.blockTiles(o=>{if(o.isLine()&&o.children.length&&o.length<=20){let a=0,l;for(let h of o.children){if(!h.isText()||/[^ -~]/.test(h.text))return;let c=Ii(h.dom);if(c.length!=1)return;a+=c[0].width,l=c[0].height}if(a)return{lineHeight:o.dom.getBoundingClientRect().height,charWidth:a/o.length,textHeight:l}}});if(e)return e;let t=document.createElement("div"),n,s,r;return t.className="cm-line",t.style.width="99999px",t.style.position="absolute",t.textContent="abc def ghi jkl mno pqr stu",this.view.observer.ignore(()=>{this.tile.dom.appendChild(t);let o=Ii(t.firstChild)[0];n=t.getBoundingClientRect().height,s=o&&o.width?o.width/27:7,r=o&&o.height?o.height:n,t.remove()}),{lineHeight:n,charWidth:s,textHeight:r}}computeBlockGapDeco(){let e=[],t=this.view.viewState;for(let n=0,s=0;;s++){let r=s==t.viewports.length?null:t.viewports[s],o=r?r.from-1:this.view.state.doc.length;if(o>n){let a=(t.lineBlockAt(o).bottom-t.lineBlockAt(n).top)/this.view.scaleY;e.push(_.replace({widget:new Hs(a),block:!0,inclusive:!0,isBlockGap:!0}).range(n,o))}if(!r)break;n=r.to+1}return _.set(e)}updateDeco(){let e=1,t=this.view.state.facet(ds).map(r=>(this.dynamicDecorationMap[e++]=typeof r=="function")?r(this.view):r),n=!1,s=this.view.state.facet(ro).map((r,o)=>{let a=typeof r=="function";return a&&(n=!0),a?r(this.view):r});for(s.length&&(this.dynamicDecorationMap[e++]=n,t.push(H.join(s))),this.decorations=[this.editContextFormatting,...t,this.computeBlockGapDeco(),this.view.viewState.lineGapDeco];e<this.decorations.length;)this.dynamicDecorationMap[e++]=!1;this.blockWrappers=this.view.state.facet(oh).map(r=>typeof r=="function"?r(this.view):r)}scrollIntoView(e){if(e.isSnapshot){let h=this.view.viewState.lineBlockAt(e.range.head);this.view.scrollDOM.scrollTop=h.top-e.yMargin,this.view.scrollDOM.scrollLeft=e.xMargin;return}for(let h of this.view.state.facet(ih))try{if(h(this.view,e.range,e))return!0}catch(c){Ce(this.view.state,c,"scroll handler")}let{range:t}=e,n=this.coordsAt(t.head,t.assoc||(t.head>t.anchor?-1:1)),s;if(!n)return;!t.empty&&(s=this.coordsAt(t.anchor,t.anchor>t.head?-1:1))&&(n={left:Math.min(n.left,s.left),top:Math.min(n.top,s.top),right:Math.max(n.right,s.right),bottom:Math.max(n.bottom,s.bottom)});let r=oo(this.view),o={left:n.left-r.left,top:n.top-r.top,right:n.right+r.right,bottom:n.bottom+r.bottom},{offsetWidth:a,offsetHeight:l}=this.view.scrollDOM;if(Gd(this.view.scrollDOM,o,t.head<t.anchor?-1:1,e.x,e.y,Math.max(Math.min(e.xMargin,a),-a),Math.max(Math.min(e.yMargin,l),-l),this.view.textDirection==J.LTR),window.visualViewport&&window.innerHeight-window.visualViewport.height>1&&(n.top>window.visualViewport.offsetTop+window.visualViewport.height||n.bottom<window.visualViewport.offsetTop)){let h=this.view.docView.lineAt(t.head,1);if(h){let c=Wl(h.dom);h.dom.scrollIntoView({block:"nearest"}),ql(c,!1)}}}lineHasWidget(e){let t=n=>n.isWidget()||n.children.some(t);return t(this.tile.resolveBlock(e,1).tile)}destroy(){Or(this.tile)}}function Or(i,e){let t=e==null?void 0:e.get(i);if(t!=1){t==null&&i.destroy();for(let n of i.children)Or(n,e)}}function vu(i){return i.node.nodeType==1&&i.node.firstChild&&(i.offset==0||i.node.childNodes[i.offset-1].contentEditable=="false")&&(i.offset==i.node.childNodes.length||i.node.childNodes[i.offset].contentEditable=="false")}function ch(i,e){let t=i.observer.selectionRange;if(!t.focusNode)return null;let n=Vl(t.focusNode,t.focusOffset),s=zl(t.focusNode,t.focusOffset),r=n||s;if(s&&n&&s.node!=n.node){let a=te.get(s.node);if(!a||a.isText()&&a.text!=s.node.nodeValue)r=s;else if(i.docView.lastCompositionAfterCursor){let l=te.get(n.node);!l||l.isText()&&l.text!=n.node.nodeValue||(r=s)}}if(i.docView.lastCompositionAfterCursor=r!=n,!r)return null;let o=e-r.offset;return{from:o,to:o+r.node.nodeValue.length,node:r.node}}function ku(i,e,t){let n=ch(i,t);if(!n)return null;let{node:s,from:r,to:o}=n,a=s.nodeValue;if(/[\n\r]/.test(a)||i.state.doc.sliceString(n.from,n.to)!=a)return null;let l=e.invertedDesc;return{range:new qe(l.mapPos(r),l.mapPos(o),r,o),text:s}}function xu(i,e){return i.nodeType!=1?0:(e&&i.childNodes[e-1].contentEditable=="false"?1:0)|(e<i.childNodes.length&&i.childNodes[e].contentEditable=="false"?2:0)}let Su=class{constructor(){this.changes=[]}compareRange(e,t){Zt(e,t,this.changes)}comparePoint(e,t){Zt(e,t,this.changes)}boundChange(e){Zt(e,e,this.changes)}};function Cu(i,e,t){let n=new Su;return H.compare(i,e,t,n),n.changes}class Au{constructor(){this.changes=[]}compareRange(e,t){Zt(e,t,this.changes)}comparePoint(){}boundChange(e){Zt(e,e,this.changes)}}function Tu(i,e,t){let n=new Au;return H.compare(i,e,t,n),n.changes}function Mu(i,e){for(let t=i;t&&t!=e;t=t.assignedSlot||t.parentNode)if(t.nodeType==1&&t.contentEditable=="false")return!0;return!1}function Ou(i,e){let t=!1;return e&&i.iterChangedRanges((n,s)=>{n<e.to&&s>e.from&&(t=!0)}),t}class Hs extends Xn{constructor(e){super(),this.height=e}toDOM(){let e=document.createElement("div");return e.className="cm-gap",this.updateDOM(e),e}eq(e){return e.height==this.height}updateDOM(e){return e.style.height=this.height+"px",!0}get editable(){return!0}get estimatedHeight(){return this.height}ignoreEvent(){return!1}}function Iu(i,e,t=1){let n=i.charCategorizer(e),s=i.doc.lineAt(e),r=e-s.from;if(s.length==0)return b.cursor(e);r==0?t=1:r==s.length&&(t=-1);let o=r,a=r;t<0?o=he(s.text,r,!1):a=he(s.text,r);let l=n(s.text.slice(o,a));for(;o>0;){let h=he(s.text,o,!1);if(n(s.text.slice(h,o))!=l)break;o=h}for(;a<s.length;){let h=he(s.text,a);if(n(s.text.slice(a,h))!=l)break;a=h}return b.undirectionalRange(o+s.from,a+s.from)}function Du(i,e,t,n,s){let r=Math.round((n-e.left)*i.defaultCharacterWidth);if(i.lineWrapping&&t.height>i.defaultLineHeight*1.5){let a=i.viewState.heightOracle.textHeight,l=Math.floor((s-t.top-(i.defaultLineHeight-a)*.5)/a);r+=l*i.viewState.heightOracle.lineLength}let o=i.state.sliceDoc(t.from,t.to);return t.from+qd(o,r,i.state.tabSize)}function Ir(i,e,t){let n=i.lineBlockAt(e);if(Array.isArray(n.type)){let s;for(let r of n.type){if(r.from>e)break;if(!(r.to<e)){if(r.from<e&&r.to>e)return r;(!s||r.type==pe.Text&&(s.type!=r.type||(t<0?r.from<e:r.to>e)))&&(s=r)}}return s||n}return n}function Ru(i,e,t,n){let s=Ir(i,e.head,e.assoc||-1),r=!n||s.type!=pe.Text||!(i.lineWrapping||s.widgetLineBreaks)?null:i.coordsAtPos(e.assoc<0&&e.head>s.from?e.head-1:e.head);if(r){let o=i.dom.getBoundingClientRect(),a=i.textDirectionAt(s.from),l=i.posAtCoords({x:t==(a==J.LTR)?o.right-1:o.left+1,y:(r.top+r.bottom)/2});if(l!=null)return b.cursor(l,t?-1:1)}return b.cursor(t?s.to:s.from,t?-1:1)}function ta(i,e,t,n){let s=i.state.doc.lineAt(e.head),r=i.bidiSpans(s),o=i.textDirectionAt(s.from);for(let a=e,l=null;;){let h=ru(s,r,o,a,t),c=Gl;if(!h){if(s.number==(t?i.state.doc.lines:1))return a;c=`
`,s=i.state.doc.line(s.number+(t?1:-1)),r=i.bidiSpans(s),h=t?b.cursor(s.from,-1):b.cursor(s.to,1)}if(l){if(!l(c))return a}else{if(!n)return h;l=n(c)}a=h}}function Eu(i,e,t){let n=i.state.charCategorizer(e),s=n(t);return r=>{let o=n(r);return s==Z.Space&&(s=o),s==o}}function Lu(i,e,t,n){let s=e.head,r=t?1:-1;if(s==(t?i.state.doc.length:0))return b.cursor(s,e.assoc);let o=e.goalColumn,a,l=i.contentDOM.getBoundingClientRect(),h=i.coordsAtPos(s,e.assoc||((e.empty?t:e.head==e.from)?1:-1)),c=i.documentTop;if(h)o==null&&(o=h.left-l.left),a=r<0?h.top:h.bottom;else{let p=i.viewState.lineBlockAt(s);o==null&&(o=Math.min(l.right-l.left,i.defaultCharacterWidth*(s-p.from))),a=(r<0?p.top:p.bottom)+c}let d=l.left+o,u=i.viewState.heightOracle.textHeight>>1,f=n??u;for(let p=0;;p+=u){let m=a+(f+p)*r,g=Dr(i,{x:d,y:m},!1,r);if(t?m>l.bottom:m<l.top)return b.cursor(g.pos,g.assoc);let y=i.coordsAtPos(g.pos,g.assoc),x=y?(y.top+y.bottom)/2:0;if(!y||(t?x>a:x<a))return b.cursor(g.pos,g.assoc,void 0,o)}}function Bn(i,e,t){for(;;){let n=0;for(let s of i)s.between(e-1,e+1,(r,o,a)=>{if(e>r&&e<o){let l=n||t||(e-r<o-e?-1:1);e=l<0?r:o,n=l}});if(!n)return e}}function dh(i,e){let t=null;for(let n=0;n<e.ranges.length;n++){let s=e.ranges[n],r=null;if(s.empty){let o=Bn(i,s.from,0);o!=s.from&&(r=b.cursor(o,-1))}else{let o=Bn(i,s.from,-1),a=Bn(i,s.to,1);(o!=s.from||a!=s.to)&&(s.undirectional?r=b.undirectionalRange(s.from,s.to):r=b.range(s.from==s.anchor?o:a,s.from==s.head?o:a))}r&&(t||(t=e.ranges.slice()),t[n]=r)}return t?b.create(t,e.mainIndex):e}function Fs(i,e,t){let n=Bn(i.state.facet(ei).map(s=>s(i)),t.from,e.head>t.from?-1:1);return n==t.from?t:b.cursor(n,n<t.from?1:-1)}class ot{constructor(e,t){this.pos=e,this.assoc=t}}function Dr(i,e,t,n){let s=i.contentDOM.getBoundingClientRect(),r=s.top+i.viewState.paddingTop,{x:o,y:a}=e,l=a-r,h;for(;;){if(l<0)return new ot(0,1);if(l>i.viewState.docHeight)return new ot(i.state.doc.length,-1);if(h=i.elementAtHeight(l),n==null)break;if(h.type==pe.Text){if(n<0?h.to<i.viewport.from:h.from>i.viewport.to)break;let u=i.docView.coordsAt(n<0?h.from:h.to,n>0?-1:1);if(u&&(n<0?u.top<=l+r:u.bottom>=l+r))break}let d=i.viewState.heightOracle.textHeight/2;l=n>0?h.bottom+d:h.top-d}if(i.viewport.from>=h.to||i.viewport.to<=h.from){if(t)return null;if(h.type==pe.Text){let d=Du(i,s,h,o,a);return new ot(d,d==h.from?1:-1)}}if(h.type!=pe.Text)return l<(h.top+h.bottom)/2?new ot(h.from,1):new ot(h.to,-1);let c=i.docView.lineAt(h.from,2);return(!c||c.length!=h.length)&&(c=i.docView.lineAt(h.from,-2)),new Bu(i,o,a,i.textDirectionAt(h.from)).scanTile(c,h.from)}class Bu{constructor(e,t,n,s){this.view=e,this.x=t,this.y=n,this.baseDir=s,this.line=null,this.spans=null}bidiSpansAt(e){return(!this.line||this.line.from>e||this.line.to<e)&&(this.line=this.view.state.doc.lineAt(e),this.spans=this.view.bidiSpans(this.line)),this}baseDirAt(e,t){let{line:n,spans:s}=this.bidiSpansAt(e);return s[at.find(s,e-n.from,-1,t)].level==this.baseDir}dirAt(e,t){let{line:n,spans:s}=this.bidiSpansAt(e);return s[at.find(s,e-n.from,-1,t)].dir}bidiIn(e,t){let{spans:n,line:s}=this.bidiSpansAt(e);return n.length>1||n.length&&(n[0].level!=this.baseDir||n[0].to+s.from<t)}scan(e,t,n=!1){let s=0,r=e.length-1,o=new Set,a=this.bidiIn(e[0],e[r]),l,h,c=-1,d=1e9,u;e:for(;s<r;){let p=r-s,m=s+r>>1;t:if(o.has(m)){for(let x=1;x<p;x++){let v=m+x;if(v>=r&&(v-=p),!o.has(v)){m=v;break t}}break e}o.add(m);let g=t(m),y=0;if(g)for(let x=0;x<g.length;x++){let v=g[x];if(!(v.width==0&&g.length>1))if(v.bottom<this.y)(!l||l.bottom<v.bottom)&&(l=v),y=1;else if(v.top>this.y)(!h||h.top>v.top)&&(h=v),y=-1;else{let I=v.left>this.x?this.x-v.left:v.right<this.x?this.x-v.right:0,S=Math.abs(I);S<d&&(c=m,d=S,u=v),I&&(y=I<0==(this.baseDir==J.LTR)?-1:1)}}y==-1&&(!a||this.baseDirAt(e[m],1))?r=m:y==1&&(!a||this.baseDirAt(e[m+1],-1))&&(s=m+1)}if(!u){if(!h&&!l)return{i:0,after:!1};let p=l&&(!h||this.y-l.bottom<h.top-this.y)?l:h;return this.y=(p.top+p.bottom)/2,this.scan(e,t,!0)}if(d&&!n){let{top:p,bottom:m}=u;if(l&&l.bottom>(p+p+m)/3)return this.y=l.bottom-1,this.scan(e,t,!0);if(h&&h.top<(p+m+m)/3)return this.y=h.top+1,this.scan(e,t,!0)}let f=(a?this.dirAt(e[c],1):this.baseDir)==J.LTR;return{i:c,after:this.x>(u.left+u.right)/2==f}}scanText(e,t){let n=[];for(let r=0;r<e.length;r=he(e.text,r))n.push(t+r);n.push(t+e.length);let s=this.scan(n,r=>{let o=n[r]-t,a=n[r+1]-t;return jn(e.dom,o,a).getClientRects()});return s.after?new ot(n[s.i+1],-1):new ot(n[s.i],1)}scanTile(e,t){if(!e.length)return new ot(t,1);if(e.children.length==1){let a=e.children[0];if(a.isText())return this.scanText(a,t);if(a.isComposite())return this.scanTile(a,t)}let n=[t];for(let a=0,l=t;a<e.children.length;a++)n.push(l+=e.children[a].length);let s=this.scan(n,a=>{let l=e.children[a];return l.flags&48?null:(l.dom.nodeType==1?l.dom:jn(l.dom,0,l.length)).getClientRects()}),r=e.children[s.i],o=n[s.i];return r.isText()?this.scanText(r,o):r.isComposite()?this.scanTile(r,o):s.after?new ot(n[s.i+1],-1):new ot(o,1)}}const $t="￿";class _u{constructor(e,t){this.points=e,this.view=t,this.text="",this.lineSeparator=t.state.facet(z.lineSeparator)}append(e){this.text+=e}lineBreak(){this.text+=$t}readRange(e,t){if(!e)return this;let n=e.parentNode;for(let s=e;;){this.findPointBefore(n,s);let r=this.text.length;this.readNode(s);let o=te.get(s),a=s.nextSibling;if(a==t){o!=null&&o.breakAfter&&!a&&n!=this.view.contentDOM&&this.lineBreak();break}let l=te.get(a);(o&&l?o.breakAfter:(o?o.breakAfter:qi(s))||qi(a)&&(s.nodeName!="BR"||o!=null&&o.isWidget())&&this.text.length>r)&&!Nu(a,t)&&this.lineBreak(),s=a}return this.findPointBefore(n,t),this}readTextNode(e){let t=e.nodeValue;for(let n of this.points)n.node==e&&(n.pos=this.text.length+Math.min(n.offset,t.length));for(let n=0,s=this.lineSeparator?null:/\r\n?|\n/g;;){let r=-1,o=1,a;if(this.lineSeparator?(r=t.indexOf(this.lineSeparator,n),o=this.lineSeparator.length):(a=s.exec(t))&&(r=a.index,o=a[0].length),this.append(t.slice(n,r<0?t.length:r)),r<0)break;if(this.lineBreak(),o>1)for(let l of this.points)l.node==e&&l.pos>this.text.length&&(l.pos-=o-1);n=r+o}}readNode(e){let t=te.get(e),n=t&&t.overrideDOMText;if(n!=null){this.findPointInside(e,n.length);for(let s=n.iter();!s.next().done;)s.lineBreak?this.lineBreak():this.append(s.value)}else e.nodeType==3?this.readTextNode(e):e.nodeName=="BR"?e.nextSibling&&this.lineBreak():e.nodeType==1&&this.readRange(e.firstChild,null)}findPointBefore(e,t){for(let n of this.points)n.node==e&&e.childNodes[n.offset]==t&&(n.pos=this.text.length)}findPointInside(e,t){for(let n of this.points)(e.nodeType==3?n.node==e:e.contains(n.node))&&(n.pos=this.text.length+(Pu(e,n.node,n.offset)?t:0))}}function Pu(i,e,t){for(;;){if(!e||t<mt(e))return!1;if(e==i)return!0;t=It(e)+1,e=e.parentNode}}function Nu(i,e){let t;for(;!(i==e||!i);i=i.nextSibling){let n=te.get(i);if(!(n!=null&&n.isWidget()))return!1;n&&(t||(t=[])).push(n)}if(t)for(let n of t){let s=n.overrideDOMText;if(s!=null&&s.length)return!1}return!0}class na{constructor(e,t){this.node=e,this.offset=t,this.pos=-1}}class Wu{constructor(e,t,n,s){this.typeOver=s,this.bounds=null,this.text="",this.domChanged=t>-1;let{impreciseHead:r,impreciseAnchor:o}=e.docView,a=e.state.selection;if(e.state.readOnly&&t>-1)this.newSel=null;else if(t>-1&&(this.bounds=uh(e.docView.tile,t,n,0))){let l=r||o?[]:Hu(e),h=new _u(l,e);h.readRange(this.bounds.startDOM,this.bounds.endDOM),this.text=h.text,this.newSel=Fu(l,this.bounds.from)}else{let l=e.observer.selectionRange,h=r&&r.node==l.focusNode&&r.offset==l.focusOffset||!Sr(e.contentDOM,l.focusNode)?a.main.head:e.docView.posFromDOM(l.focusNode,l.focusOffset),c=o&&o.node==l.anchorNode&&o.offset==l.anchorOffset||!Sr(e.contentDOM,l.anchorNode)?a.main.anchor:e.docView.posFromDOM(l.anchorNode,l.anchorOffset),d=e.viewport;if((O.ios||O.chrome)&&h!=c&&Math.min(h,c)<=a.main.from&&Math.max(h,c)>=a.main.to&&(d.from>0||d.to<e.state.doc.length)){let u=Math.min(h,c),f=Math.max(h,c),p=d.from-u,m=d.to-f;(p==0||p==1||u==0)&&(m==0||m==-1||f==e.state.doc.length)&&(h=0,c=e.state.doc.length)}if(e.inputState.composing>-1&&a.ranges.length>1)this.newSel=a.replaceRange(b.range(c,h));else if(e.lineWrapping&&c==h&&!(a.main.empty&&a.main.head==h)&&e.inputState.lastTouchTime>Date.now()-100){let u=e.coordsAtPos(h,-1),f=0;u&&(f=e.inputState.lastTouchY<=u.bottom?-1:1),this.newSel=b.create([b.cursor(h,f)])}else this.newSel=b.single(c,h)}}}function uh(i,e,t,n){if(i.isComposite()){let s=-1,r=-1,o=-1,a=-1;for(let l=0,h=n,c=n;l<i.children.length;l++){let d=i.children[l],u=h+d.length;if(h<e&&u>t)return uh(d,e,t,h);if(u>=e&&s==-1&&(s=l,r=h),h>t&&d.dom.parentNode==i.dom){o=l,a=c;break}c=u,h=u+d.breakAfter}return{from:r,to:a<0?n+i.length:a,startDOM:(s?i.children[s-1].dom.nextSibling:null)||i.dom.firstChild,endDOM:o<i.children.length&&o>=0?i.children[o].dom:null}}else return i.isText()?{from:n,to:n+i.length,startDOM:i.dom,endDOM:i.dom.nextSibling}:null}function fh(i,e){let t,{newSel:n}=e,{state:s}=i,r=s.selection.main,o=i.inputState.lastKeyTime>Date.now()-100?i.inputState.lastKeyCode:-1;if(e.bounds){let{from:a,to:l}=e.bounds,h=r.from,c=null;(o===8||O.android&&e.text.length<l-a)&&(h=r.to,c="end");let d=s.doc.sliceString(a,l,$t),u,f;!r.empty&&r.from>=a&&r.to<=l&&(e.typeOver||d!=e.text)&&d.slice(0,r.from-a)==e.text.slice(0,r.from-a)&&d.slice(r.to-a)==e.text.slice(u=e.text.length-(d.length-(r.to-a)))?t={from:r.from,to:r.to,insert:j.of(e.text.slice(r.from-a,u).split($t))}:(f=ph(d,e.text,h-a,c))&&(O.chrome&&o==13&&f.toB==f.from+2&&e.text.slice(f.from,f.toB)==$t+$t&&f.toB--,t={from:a+f.from,to:a+f.toA,insert:j.of(e.text.slice(f.from,f.toB).split($t))})}else n&&(!i.hasFocus&&s.facet(ut)||ji(n,r))&&(n=null);if(!t&&!n)return!1;if((O.mac||O.android)&&t&&t.from==t.to&&t.from==r.head-1&&/^\. ?$/.test(t.insert.toString())&&i.contentDOM.getAttribute("autocorrect")=="off"?(n&&t.insert.length==2&&(n=b.single(n.main.anchor-1,n.main.head-1)),t={from:t.from,to:t.to,insert:j.of([t.insert.toString().replace("."," ")])}):s.doc.lineAt(r.from).to<r.to&&i.docView.lineHasWidget(r.to)&&i.inputState.insertingTextAt>Date.now()-50?t={from:r.from,to:r.to,insert:s.toText(i.inputState.insertingText)}:O.chrome&&t&&t.from==t.to&&t.from==r.head&&t.insert.toString()==`
 `&&i.lineWrapping&&(n&&(n=b.single(n.main.anchor-1,n.main.head-1)),t={from:r.from,to:r.to,insert:j.of([" "])}),t)return ao(i,t,n,o);if(n&&!ji(n,r)){let a=!1,l="select";return i.inputState.lastSelectionTime>Date.now()-50&&(i.inputState.lastSelectionOrigin=="select"&&(a=!0),l=i.inputState.lastSelectionOrigin,l=="select.pointer"&&(n=dh(s.facet(ei).map(h=>h(i)),n))),i.dispatch({selection:n,scrollIntoView:a,userEvent:l}),!0}else return!1}function ao(i,e,t,n=-1){if(O.ios&&i.inputState.flushIOSKey(e))return!0;let s=i.state.selection.main;if(O.android&&(e.to==s.to&&(e.from==s.from||e.from==s.from-1&&i.state.sliceDoc(e.from,s.from)==" ")&&e.insert.length==1&&e.insert.lines==2&&en(i.contentDOM,"Enter",13)||(e.from==s.from-1&&e.to==s.to&&e.insert.length==0||n==8&&e.insert.length<e.to-e.from&&e.to>s.head)&&en(i.contentDOM,"Backspace",8)||e.from==s.from&&e.to==s.to+1&&e.insert.length==0&&en(i.contentDOM,"Delete",46)))return!0;let r=e.insert.toString();i.inputState.composing>=0&&i.inputState.composing++;let o,a=()=>o||(o=qu(i,e,t));return i.state.facet(Zl).some(l=>l(i,e.from,e.to,r,a))||i.dispatch(a()),!0}function qu(i,e,t){let n,s=i.state,r=s.selection.main,o=-1;if(e.from==e.to&&e.from<r.from||e.from>r.to){let l=e.from<r.from?-1:1,h=l<0?r.from:r.to,c=Bn(s.facet(ei).map(d=>d(i)),h,l);e.from==c&&(o=c)}if(o>-1)n={changes:e,selection:b.cursor(e.from+e.insert.length,-1)};else if(e.from>=r.from&&e.to<=r.to&&e.to-e.from>=(r.to-r.from)/3&&(!t||t.main.empty&&t.main.from==e.from+e.insert.length)&&i.inputState.composing<0){let l=r.from<e.from?s.sliceDoc(r.from,e.from):"",h=r.to>e.to?s.sliceDoc(e.to,r.to):"";n=s.replaceSelection(i.state.toText(l+e.insert.sliceString(0,void 0,i.state.lineBreak)+h))}else{let l=s.changes(e),h=t&&t.main.to<=l.newLength?t.main:void 0;if(s.selection.ranges.length>1&&(i.inputState.composing>=0||i.inputState.compositionPendingChange)&&e.to<=r.to+10&&e.to>=r.to-10){let c=i.state.sliceDoc(e.from,e.to),d,u=t&&ch(i,t.main.head);if(u){let p=e.insert.length-(e.to-e.from);d={from:u.from,to:u.to-p}}else d=i.state.doc.lineAt(r.head);let f=r.to-e.to;n=s.changeByRange(p=>{if(p.from==r.from&&p.to==r.to)return{changes:l,range:h||p.map(l)};let m=p.to-f,g=m-c.length;if(i.state.sliceDoc(g,m)!=c||m>=d.from&&g<=d.to)return{range:p};let y=s.changes({from:g,to:m,insert:e.insert}),x=p.to-r.to;return{changes:y,range:h?b.range(Math.max(0,h.anchor+x),Math.max(0,h.head+x)):p.map(y)}})}else n={changes:l,selection:h&&s.selection.replaceRange(h)}}let a="input.type";return(i.composing||i.inputState.compositionPendingChange&&i.inputState.compositionEndedAt>Date.now()-50)&&(i.inputState.compositionPendingChange=!1,a+=".compose",i.inputState.compositionFirstChange&&(a+=".start",i.inputState.compositionFirstChange=!1)),s.update(n,{userEvent:a,scrollIntoView:!0})}function ph(i,e,t,n){let s=Math.min(i.length,e.length),r=0;for(;r<s&&i.charCodeAt(r)==e.charCodeAt(r);)r++;if(r==s&&i.length==e.length)return null;let o=i.length,a=e.length;for(;o>0&&a>0&&i.charCodeAt(o-1)==e.charCodeAt(a-1);)o--,a--;if(n=="end"){let l=Math.max(0,r-Math.min(o,a));t-=o+l-r}if(o<r&&i.length<e.length){let l=t<=r&&t>=o?r-t:0;r-=l,a=r+(a-o),o=r}else if(a<r){let l=t<=r&&t>=a?r-t:0;r-=l,o=r+(o-a),a=r}return{from:r,toA:o,toB:a}}function Hu(i){let e=[];if(i.root.activeElement!=i.contentDOM)return e;let{anchorNode:t,anchorOffset:n,focusNode:s,focusOffset:r}=i.observer.selectionRange;return t&&(e.push(new na(t,n)),(s!=t||r!=n)&&e.push(new na(s,r))),e}function Fu(i,e){if(i.length==0)return null;let t=i[0].pos,n=i.length==2?i[1].pos:t;return t<0||n<0?null:t==n?b.create([b.cursor(n+e,-1)]):b.single(t+e,n+e)}function ji(i,e){return e.head==i.main.head&&e.anchor==i.main.anchor}class Vu{setSelectionOrigin(e){this.lastSelectionOrigin=e,this.lastSelectionTime=Date.now()}constructor(e){this.view=e,this.lastKeyCode=0,this.lastKeyTime=0,this.touchActive=!1,this.lastTouchTime=0,this.lastTouchX=0,this.lastTouchY=0,this.lastFocusTime=0,this.lastScrollTop=0,this.lastScrollLeft=0,this.lastWheelEvent=0,this.pendingIOSKey=void 0,this.lastIOSMomentumScroll=0,this.tabFocusMode=-1,this.lastSelectionOrigin=null,this.lastSelectionTime=0,this.lastContextMenu=0,this.scrollHandlers=[],this.handlers=Object.create(null),this.composing=-1,this.compositionFirstChange=null,this.compositionEndedAt=0,this.compositionPendingKey=!1,this.compositionPendingChange=!1,this.insertingText="",this.insertingTextAt=0,this.mouseSelection=null,this.draggedContent=null,this.handleEvent=this.handleEvent.bind(this),this.notifiedFocused=e.hasFocus,O.safari&&e.contentDOM.addEventListener("input",()=>null),O.gecko&&rf(e.contentDOM.ownerDocument)}handleEvent(e){!Xu(this.view,e)||this.ignoreDuringComposition(e)||e.type=="keydown"&&this.keydown(e)||(this.view.updateState!=0?Promise.resolve().then(()=>this.runHandlers(e.type,e)):this.runHandlers(e.type,e))}runHandlers(e,t){let n=this.handlers[e];if(n){for(let s of n.observers)s(this.view,t);for(let s of n.handlers){if(t.defaultPrevented)break;if(s(this.view,t)){t.preventDefault();break}}}}ensureHandlers(e){let t=ju(e),n=this.handlers,s=this.view.contentDOM;for(let r in t)if(r!="scroll"){let o=!t[r].handlers.length,a=n[r];a&&o!=!a.handlers.length&&(s.removeEventListener(r,this.handleEvent),a=null),a||s.addEventListener(r,this.handleEvent,{passive:o})}for(let r in n)r!="scroll"&&!t[r]&&s.removeEventListener(r,this.handleEvent);this.handlers=t}keydown(e){if(this.lastKeyCode=e.keyCode,this.lastKeyTime=Date.now(),e.keyCode==9&&this.tabFocusMode>-1&&(!this.tabFocusMode||Date.now()<=this.tabFocusMode))return!0;if(this.tabFocusMode>0&&e.keyCode!=27&&gh.indexOf(e.keyCode)<0&&(this.tabFocusMode=-1),O.android&&O.chrome&&!e.synthetic&&(e.keyCode==13||e.keyCode==8))return this.view.observer.delayAndroidKey(e.key,e.keyCode),!0;if(O.ios&&!e.synthetic&&!e.altKey&&!e.metaKey&&(mh.some(t=>t.keyCode==e.keyCode)&&!e.ctrlKey||Uu.indexOf(e.key)>-1&&e.ctrlKey)){let t={ctrlKey:e.ctrlKey,altKey:e.altKey,metaKey:e.metaKey,shiftKey:e.shiftKey};t.shiftKey&&O.ios&&!/^(off|none)$/.test(this.view.contentDOM.autocapitalize)&&zu(this.view.win)&&(t.shiftKey=!1);let n=this.pendingIOSKey={key:e.key,keyCode:e.keyCode,mods:t};return setTimeout(()=>{this.pendingIOSKey==n&&this.flushIOSKey()},50),!0}return e.keyCode!=229&&this.view.observer.forceFlush(),!1}flushIOSKey(e){let t=this.pendingIOSKey;return!t||this.view.observer.pendingRecords().length||t.key=="Enter"&&e&&e.from<e.to&&/^\S+$/.test(e.insert.toString())?!1:(this.pendingIOSKey=void 0,en(this.view.contentDOM,t.key,t.keyCode,t.mods))}ignoreDuringComposition(e){return!/^key/.test(e.type)||e.synthetic?!1:this.composing>0?!0:O.safari&&!O.ios&&this.compositionPendingKey&&Date.now()-this.compositionEndedAt<100?(this.compositionPendingKey=!1,!0):!1}startMouseSelection(e){this.mouseSelection&&this.mouseSelection.destroy(),this.mouseSelection=e}update(e){this.view.observer.update(e),this.mouseSelection&&this.mouseSelection.update(e),this.draggedContent&&e.docChanged&&(this.draggedContent=this.draggedContent.map(e.changes)),e.transactions.length&&(this.lastKeyCode=this.lastSelectionTime=0)}destroy(){this.mouseSelection&&this.mouseSelection.destroy()}}function zu(i){return i.visualViewport?i.visualViewport.height*i.visualViewport.scale/i.document.documentElement.clientHeight<.85:!1}function ia(i,e){return(t,n)=>{try{return e.call(i,n,t)}catch(s){Ce(t.state,s)}}}function ju(i){let e=Object.create(null);function t(n){return e[n]||(e[n]={observers:[],handlers:[]})}for(let n of i){let s=n.spec,r=s&&s.plugin.domEventHandlers,o=s&&s.plugin.domEventObservers;if(r)for(let a in r){let l=r[a];l&&t(a).handlers.push(ia(n.value,l))}if(o)for(let a in o){let l=o[a];l&&t(a).observers.push(ia(n.value,l))}}for(let n in Ye)t(n).handlers.push(Ye[n]);for(let n in Te)t(n).observers.push(Te[n]);return e}const mh=[{key:"Backspace",keyCode:8,inputType:"deleteContentBackward"},{key:"Enter",keyCode:13,inputType:"insertParagraph"},{key:"Enter",keyCode:13,inputType:"insertLineBreak"},{key:"Delete",keyCode:46,inputType:"deleteContentForward"}],Uu="dthko",gh=[16,17,18,20,91,92,224,225],ai=6;function li(i){return Math.max(0,i)*.7+8}function $u(i,e){return Math.max(Math.abs(i.clientX-e.clientX),Math.abs(i.clientY-e.clientY))}class Ku{constructor(e,t,n,s){this.view=e,this.startEvent=t,this.style=n,this.mustSelect=s,this.scrollSpeed={x:0,y:0},this.scrolling=-1,this.lastEvent=t,this.scrollParents=Nl(e.contentDOM),this.atoms=e.state.facet(ei).map(o=>o(e));let r=e.contentDOM.ownerDocument;r.addEventListener("mousemove",this.move=this.move.bind(this)),r.addEventListener("mouseup",this.up=this.up.bind(this)),this.extend=t.shiftKey,this.multiple=e.state.facet(z.allowMultipleSelections)&&Gu(e,t),this.dragging=Ju(e,t)&&wh(t)==1?null:!1}start(e){this.dragging===!1&&this.select(e)}move(e){if(e.buttons==0)return this.destroy();if(this.dragging||this.dragging==null&&$u(this.startEvent,e)<10)return;this.select(this.lastEvent=e);let t=0,n=0,s=0,r=0,o=this.view.win.innerWidth,a=this.view.win.innerHeight;this.scrollParents.x&&({left:s,right:o}=this.scrollParents.x.getBoundingClientRect()),this.scrollParents.y&&({top:r,bottom:a}=this.scrollParents.y.getBoundingClientRect());let l=oo(this.view);e.clientX-l.left<=s+ai?t=-li(s-e.clientX):e.clientX+l.right>=o-ai&&(t=li(e.clientX-o)),e.clientY-l.top<=r+ai?n=-li(r-e.clientY):e.clientY+l.bottom>=a-ai&&(n=li(e.clientY-a)),this.setScrollSpeed(t,n)}up(e){this.dragging==null&&this.select(this.lastEvent),this.dragging||e.preventDefault(),this.destroy()}destroy(){this.setScrollSpeed(0,0);let e=this.view.contentDOM.ownerDocument;e.removeEventListener("mousemove",this.move),e.removeEventListener("mouseup",this.up),this.view.inputState.mouseSelection=this.view.inputState.draggedContent=null}setScrollSpeed(e,t){this.scrollSpeed={x:e,y:t},e||t?this.scrolling<0&&(this.scrolling=setInterval(()=>this.scroll(),50)):this.scrolling>-1&&(clearInterval(this.scrolling),this.scrolling=-1)}scroll(){let{x:e,y:t}=this.scrollSpeed;e&&this.scrollParents.x&&(this.scrollParents.x.scrollLeft+=e,e=0),t&&this.scrollParents.y&&(this.scrollParents.y.scrollTop+=t,t=0),(e||t)&&this.view.win.scrollBy(e,t),this.dragging===!1&&this.select(this.lastEvent)}select(e){let{view:t}=this,n=dh(this.atoms,this.style.get(e,this.extend,this.multiple));(this.mustSelect||!n.eq(t.state.selection,this.dragging===!1))&&this.view.dispatch({selection:n,userEvent:"select.pointer"}),this.mustSelect=!1}update(e){e.transactions.some(t=>t.isUserEvent("input.type"))?this.destroy():this.style.update(e)&&setTimeout(()=>this.select(this.lastEvent),20)}}function Gu(i,e){let t=i.state.facet(Yl);return t.length?t[0](e):O.mac?e.metaKey:e.ctrlKey}function Yu(i,e){let t=i.state.facet(Jl);return t.length?t[0](e):O.mac?!e.altKey:!e.ctrlKey}function Ju(i,e){let{main:t}=i.state.selection;if(t.empty)return!1;let n=zn(i.root);if(!n||n.rangeCount==0)return!0;let s=n.getRangeAt(0).getClientRects();for(let r=0;r<s.length;r++){let o=s[r];if(o.left<=e.clientX&&o.right>=e.clientX&&o.top<=e.clientY&&o.bottom>=e.clientY)return!0}return!1}function Xu(i,e){if(!e.bubbles)return!0;if(e.defaultPrevented)return!1;for(let t=e.target,n;t!=i.contentDOM;t=t.parentNode)if(!t||t.nodeType==11||(n=te.get(t))&&n.isWidget()&&!n.isHidden&&n.widget.ignoreEvent(e))return!1;return!0}const Ye=Object.create(null),Te=Object.create(null),yh=O.ie&&O.ie_version<15||O.ios&&O.webkit_version<604;function Qu(i){let e=i.dom.parentNode;if(!e)return;let t=e.appendChild(document.createElement("textarea"));t.style.cssText="position: fixed; left: -10000px; top: 10px",t.focus(),setTimeout(()=>{i.focus(),t.remove(),bh(i,t.value)},50)}function ps(i,e,t){for(let n of i.facet(e))t=n(t,i);return t}function bh(i,e){e=ps(i.state,no,e);let{state:t}=i,n,s=1,r=t.toText(e),o=r.lines==t.selection.ranges.length;if(Rr!=null&&t.selection.ranges.every(l=>l.empty)&&Rr==r.toString()){let l=-1;n=t.changeByRange(h=>{let c=t.doc.lineAt(h.from);if(c.from==l)return{range:h};l=c.from;let d=t.toText((o?r.line(s++).text:e)+t.lineBreak);return{changes:{from:c.from,insert:d},range:b.cursor(h.from+d.length,-1)}})}else o?n=t.changeByRange(l=>{let h=r.line(s++);return{changes:{from:l.from,to:l.to,insert:h.text},range:b.cursor(l.from+h.length,-1)}}):n=t.replaceSelection(r);i.dispatch(n,{userEvent:"input.paste",scrollIntoView:!0})}Te.scroll=i=>{let e=i.inputState;e.lastScrollTop=i.scrollDOM.scrollTop,e.lastScrollLeft=i.scrollDOM.scrollLeft,O.ios&&!e.touchActive&&(e.lastIOSMomentumScroll=Date.now())};Te.wheel=Te.mousewheel=i=>{i.inputState.lastWheelEvent=Date.now()};Ye.keydown=(i,e)=>(i.inputState.setSelectionOrigin("select"),e.keyCode==27&&i.inputState.tabFocusMode!=0&&(i.inputState.tabFocusMode=Date.now()+2e3),!1);Te.touchstart=(i,e)=>{let t=i.inputState,n=e.targetTouches[0];t.touchActive=!0,t.lastTouchTime=Date.now(),n&&(t.lastTouchX=n.clientX,t.lastTouchY=n.clientY),t.setSelectionOrigin("select.pointer")};Te.touchmove=i=>{i.inputState.setSelectionOrigin("select.pointer")};Te.touchend=(i,e)=>{i.inputState.touchActive=!1};Ye.mousedown=(i,e)=>{if(i.observer.flush(),i.inputState.lastTouchTime>Date.now()-2e3)return!1;let t=null;for(let n of i.state.facet(Xl))if(t=n(i,e),t)break;if(!t&&e.button==0&&(t=ef(i,e)),t){let n=!i.hasFocus;i.inputState.startMouseSelection(new Ku(i,e,t,n)),n&&i.observer.ignore(()=>{Hl(i.contentDOM);let r=i.root.activeElement;r&&!r.contains(i.contentDOM)&&r.blur()});let s=i.inputState.mouseSelection;if(s)return s.start(e),s.dragging===!1}else i.inputState.setSelectionOrigin("select.pointer");return!1};function sa(i,e,t,n){if(n==1)return b.cursor(e,t);if(n==2)return Iu(i.state,e,t);{let s=i.docView.lineAt(e,t),r=i.state.doc.lineAt(s?s.posAtEnd:e),o=s?s.posAtStart:r.from,a=s?s.posAtEnd:r.to;return a<i.state.doc.length&&a==r.to&&a++,b.undirectionalRange(o,a)}}const Zu=O.ie&&O.ie_version<=11;let ra=null,oa=0,aa=0;function wh(i){if(!Zu)return i.detail;let e=ra,t=aa;return ra=i,aa=Date.now(),oa=!e||t>Date.now()-400&&Math.abs(e.clientX-i.clientX)<2&&Math.abs(e.clientY-i.clientY)<2?(oa+1)%3:1}function ef(i,e){let t=i.posAndSideAtCoords({x:e.clientX,y:e.clientY},!1),n=wh(e),s=i.state.selection;return{update(r){r.docChanged&&(t.pos=r.changes.mapPos(t.pos),s=s.map(r.changes))},get(r,o,a){let l=i.posAndSideAtCoords({x:r.clientX,y:r.clientY},!1),h,c=sa(i,l.pos,l.assoc,n);if(t.pos!=l.pos&&!o){let d=sa(i,t.pos,t.assoc,n),u=Math.min(d.from,c.from),f=Math.max(d.to,c.to);c=u<c.from?b.range(u,f,c.assoc):b.range(f,u,c.assoc)}return o?s.replaceRange(s.main.extend(c.from,c.to,c.assoc)):a&&n==1&&s.ranges.length>1&&(h=tf(s,l.pos))?h:a?s.addRange(c):b.create([c])}}}function tf(i,e){for(let t=0;t<i.ranges.length;t++){let{from:n,to:s}=i.ranges[t];if(n<=e&&s>=e)return b.create(i.ranges.slice(0,t).concat(i.ranges.slice(t+1)),i.mainIndex==t?0:i.mainIndex-(i.mainIndex>t?1:0))}return null}Ye.dragstart=(i,e)=>{let{selection:{main:t}}=i.state;if(e.target.draggable){let s=i.docView.tile.nearest(e.target);if(s&&s.isWidget()){let r=s.posAtStart,o=r+s.length;(r>=t.to||o<=t.from)&&(t=b.undirectionalRange(r,o))}}let{inputState:n}=i;return n.mouseSelection&&(n.mouseSelection.dragging=!0),n.draggedContent=t,e.dataTransfer&&(e.dataTransfer.setData("Text",ps(i.state,io,i.state.sliceDoc(t.from,t.to))),e.dataTransfer.effectAllowed="copyMove"),!1};Ye.dragend=i=>(i.inputState.draggedContent=null,!1);function la(i,e,t,n){if(t=ps(i.state,no,t),!t)return;let s=i.posAtCoords({x:e.clientX,y:e.clientY},!1),{draggedContent:r}=i.inputState,o=n&&r&&Yu(i,e)?{from:r.from,to:r.to}:null,a={from:s,insert:t},l=i.state.changes(o?[o,a]:a);i.focus(),i.dispatch({changes:l,selection:{anchor:l.mapPos(s,-1),head:l.mapPos(s,1)},userEvent:o?"move.drop":"input.drop"}),i.inputState.draggedContent=null}Ye.drop=(i,e)=>{if(!e.dataTransfer)return!1;if(i.state.readOnly)return!0;let t=e.dataTransfer.files;if(t&&t.length){let n=Array(t.length),s=0,r=()=>{++s==t.length&&la(i,e,n.filter(o=>o!=null).join(i.state.lineBreak),!1)};for(let o=0;o<t.length;o++){let a=new FileReader;a.onerror=r,a.onload=()=>{/[\x00-\x08\x0e-\x1f]{2}/.test(a.result)||(n[o]=a.result),r()},a.readAsText(t[o])}return!0}else{let n=e.dataTransfer.getData("Text");if(n)return la(i,e,n,!0),!0}return!1};Ye.paste=(i,e)=>{if(i.state.readOnly)return!0;i.observer.flush();let t=yh?null:e.clipboardData;return t?(bh(i,t.getData("text/plain")||t.getData("text/uri-list")),!0):(Qu(i),!1)};function nf(i,e){let t=i.dom.parentNode;if(!t)return;let n=t.appendChild(document.createElement("textarea"));n.style.cssText="position: fixed; left: -10000px; top: 10px",n.value=e,n.focus(),n.selectionEnd=e.length,n.selectionStart=0,setTimeout(()=>{n.remove(),i.focus()},50)}function sf(i){let e=[],t=[],n=!1;for(let s of i.selection.ranges)s.empty||(e.push(i.sliceDoc(s.from,s.to)),t.push(s));if(!e.length){let s=-1;for(let{from:r}of i.selection.ranges){let o=i.doc.lineAt(r);o.number>s&&(e.push(o.text),t.push({from:o.from,to:Math.min(i.doc.length,o.to+1)})),s=o.number}n=!0}return{text:ps(i,io,e.join(i.lineBreak)),ranges:t,linewise:n}}let Rr=null;Ye.copy=Ye.cut=(i,e)=>{if(!En(i.contentDOM,i.observer.selectionRange))return!1;let{text:t,ranges:n,linewise:s}=sf(i.state);if(!t&&!s)return!1;Rr=s?t:null,e.type=="cut"&&!i.state.readOnly&&i.dispatch({changes:n,scrollIntoView:!0,userEvent:"delete.cut"});let r=yh?null:e.clipboardData;return r?(r.clearData(),r.setData("text/plain",t),!0):(nf(i,t),!1)};const vh=yt.define();function kh(i,e){let t=[];for(let n of i.facet(eh)){let s=n(i,e);s&&t.push(s)}return t.length?i.update({effects:t,annotations:vh.of(!0)}):null}function xh(i){setTimeout(()=>{let e=i.hasFocus;if(e!=i.inputState.notifiedFocused){let t=kh(i.state,e);t?i.dispatch(t):i.update([])}},10)}Te.focus=i=>{i.inputState.lastFocusTime=Date.now(),!i.scrollDOM.scrollTop&&(i.inputState.lastScrollTop||i.inputState.lastScrollLeft)&&(i.scrollDOM.scrollTop=i.inputState.lastScrollTop,i.scrollDOM.scrollLeft=i.inputState.lastScrollLeft),xh(i)};Te.blur=i=>{i.observer.clearSelectionRange(),xh(i)};Te.compositionstart=Te.compositionupdate=i=>{if(!i.observer.editContext&&(i.inputState.compositionFirstChange==null&&(i.inputState.compositionFirstChange=!0),i.inputState.composing<0)){let{main:e}=i.state.selection;!e.empty&&i.lineBlockAt(e.from).from!=i.lineBlockAt(e.to).from&&i.dispatch({changes:i.state.selection.ranges.filter(t=>!t.empty).map(t=>({from:t.from,to:t.to})),userEvent:"input"}),i.inputState.composing=0}};Te.compositionend=i=>{i.observer.editContext||(i.inputState.composing=-1,i.inputState.compositionEndedAt=Date.now(),i.inputState.compositionPendingKey=!0,i.inputState.compositionPendingChange=i.observer.pendingRecords().length>0,i.inputState.compositionFirstChange=null,O.chrome&&O.android?i.observer.flushSoon():i.inputState.compositionPendingChange?Promise.resolve().then(()=>i.observer.flush()):setTimeout(()=>{i.inputState.composing<0&&i.docView.hasComposition&&i.update([])},50))};Te.contextmenu=i=>{i.inputState.lastContextMenu=Date.now()};Ye.beforeinput=(i,e)=>{var t,n;if((e.inputType=="insertText"||e.inputType=="insertCompositionText")&&(i.inputState.insertingText=e.data,i.inputState.insertingTextAt=Date.now()),e.inputType=="insertReplacementText"&&i.observer.editContext){let r=(t=e.dataTransfer)===null||t===void 0?void 0:t.getData("text/plain"),o=e.getTargetRanges();if(r&&o.length){let a=o[0],l=i.posAtDOM(a.startContainer,a.startOffset),h=i.posAtDOM(a.endContainer,a.endOffset);return ao(i,{from:l,to:h,insert:i.state.toText(r)},null),!0}}let s;if(O.chrome&&O.android&&(s=mh.find(r=>r.inputType==e.inputType))&&(i.observer.delayAndroidKey(s.key,s.keyCode),s.key=="Backspace"||s.key=="Delete")){let r=((n=window.visualViewport)===null||n===void 0?void 0:n.height)||0;setTimeout(()=>{var o;(((o=window.visualViewport)===null||o===void 0?void 0:o.height)||0)>r+10&&i.hasFocus&&(i.contentDOM.blur(),i.focus())},100)}return O.ios&&e.inputType=="deleteContentForward"&&i.observer.flushSoon(),O.safari&&e.inputType=="insertText"&&i.inputState.composing>=0&&setTimeout(()=>Te.compositionend(i,e),20),!1};const ha=new Set;function rf(i){ha.has(i)||(ha.add(i),i.addEventListener("copy",()=>{}),i.addEventListener("cut",()=>{}))}const ca=["pre-wrap","normal","pre-line","break-spaces"];let hn=!1;function da(){hn=!1}class of{constructor(e){this.lineWrapping=e,this.doc=j.empty,this.heightSamples={},this.lineHeight=14,this.charWidth=7,this.textHeight=14,this.lineLength=30}heightForGap(e,t){let n=this.doc.lineAt(t).number-this.doc.lineAt(e).number+1;return this.lineWrapping&&(n+=Math.max(0,Math.ceil((t-e-n*this.lineLength*.5)/this.lineLength))),this.lineHeight*n}heightForLine(e){return this.lineWrapping?(1+Math.max(0,Math.ceil((e-this.lineLength)/Math.max(1,this.lineLength-5))))*this.lineHeight:this.lineHeight}setDoc(e){return this.doc=e,this}mustRefreshForWrapping(e){return ca.indexOf(e)>-1!=this.lineWrapping}mustRefreshForHeights(e){let t=!1;for(let n=0;n<e.length;n++){let s=e[n];s<0?n++:this.heightSamples[Math.floor(s*10)]||(t=!0,this.heightSamples[Math.floor(s*10)]=!0)}return t}refresh(e,t,n,s,r,o){let a=ca.indexOf(e)>-1,l=Math.abs(t-this.lineHeight)>.3||this.lineWrapping!=a;if(this.lineWrapping=a,this.lineHeight=t,this.charWidth=n,this.textHeight=s,this.lineLength=r,l){this.heightSamples={};for(let h=0;h<o.length;h++){let c=o[h];c<0?h++:this.heightSamples[Math.floor(c*10)]=!0}}return l}}class af{constructor(e,t){this.from=e,this.heights=t,this.index=0}get more(){return this.index<this.heights.length}}class Ke{constructor(e,t,n,s,r){this.from=e,this.length=t,this.top=n,this.height=s,this._content=r}get type(){return typeof this._content=="number"?pe.Text:Array.isArray(this._content)?this._content:this._content.type}get to(){return this.from+this.length}get bottom(){return this.top+this.height}get widget(){return this._content instanceof Vt?this._content.widget:null}get widgetLineBreaks(){return typeof this._content=="number"?this._content:0}join(e){let t=(Array.isArray(this._content)?this._content:[this]).concat(Array.isArray(e._content)?e._content:[e]);return new Ke(this.from,this.length+e.length,this.top,this.height+e.height,t)}}var X=function(i){return i[i.ByPos=0]="ByPos",i[i.ByHeight=1]="ByHeight",i[i.ByPosNoHeight=2]="ByPosNoHeight",i}(X||(X={}));const Di=.001;class Ae{constructor(e,t,n=2){this.length=e,this.height=t,this.flags=n}get outdated(){return(this.flags&2)>0}set outdated(e){this.flags=(e?2:0)|this.flags&-3}setHeight(e){this.height!=e&&(Math.abs(this.height-e)>Di&&(hn=!0),this.height=e)}replace(e,t,n){return Ae.of(n)}decomposeLeft(e,t){t.push(this)}decomposeRight(e,t){t.push(this)}applyChanges(e,t,n,s){let r=this,o=n.doc;for(let a=s.length-1;a>=0;a--){let{fromA:l,toA:h,fromB:c,toB:d}=s[a],u=r.lineAt(l,X.ByPosNoHeight,n.setDoc(t),0,0),f=u.to>=h?u:r.lineAt(h,X.ByPosNoHeight,n,0,0);for(d+=f.to-h,h=f.to;a>0&&u.from<=s[a-1].toA;)l=s[a-1].fromA,c=s[a-1].fromB,a--,l<u.from&&(u=r.lineAt(l,X.ByPosNoHeight,n,0,0));c+=u.from-l,l=u.from;let p=lo.build(n.setDoc(o),e,c,d);r=Ui(r,r.replace(l,h,p))}return r.updateHeight(n,0)}static empty(){return new Be(0,0,0)}static of(e){if(e.length==1)return e[0];let t=0,n=e.length,s=0,r=0;for(;;)if(t==n)if(s>r*2){let a=e[t-1];a.break?e.splice(--t,1,a.left,null,a.right):e.splice(--t,1,a.left,a.right),n+=1+a.break,s-=a.size}else if(r>s*2){let a=e[n];a.break?e.splice(n,1,a.left,null,a.right):e.splice(n,1,a.left,a.right),n+=2+a.break,r-=a.size}else break;else if(s<r){let a=e[t++];a&&(s+=a.size)}else{let a=e[--n];a&&(r+=a.size)}let o=!1;return e[t-1]==null?(o=!0,t--):e[t]==null&&(o=!0,n++),new hf(Ae.of(e.slice(0,t)),o,Ae.of(e.slice(n)))}}function Ui(i,e){return i==e?i:(i.constructor!=e.constructor&&(hn=!0),e)}Ae.prototype.size=1;const lf=_.replace({});class Sh extends Ae{constructor(e,t,n){super(e,t),this.deco=n,this.spaceAbove=0}mainBlock(e,t){return new Ke(t,this.length,e+this.spaceAbove,this.height-this.spaceAbove,this.deco||0)}blockAt(e,t,n,s){return this.spaceAbove&&e<n+this.spaceAbove?new Ke(s,0,n,this.spaceAbove,lf):this.mainBlock(n,s)}lineAt(e,t,n,s,r){let o=this.mainBlock(s,r);return this.spaceAbove?this.blockAt(0,n,s,r).join(o):o}forEachLine(e,t,n,s,r,o){e<=r+this.length&&t>=r&&o(this.lineAt(0,X.ByPos,n,s,r))}setMeasuredHeight(e){let t=e.heights[e.index++];t<0?(this.spaceAbove=-t,t=e.heights[e.index++]):this.spaceAbove=0,this.setHeight(t)}updateHeight(e,t=0,n=!1,s){return s&&s.from<=t&&s.more&&this.setMeasuredHeight(s),this.outdated=!1,this}toString(){return`block(${this.length})`}}class Be extends Sh{constructor(e,t,n){super(e,t,null),this.collapsed=0,this.widgetHeight=0,this.breaks=0,this.spaceAbove=n}mainBlock(e,t){return new Ke(t,this.length,e+this.spaceAbove,this.height-this.spaceAbove,this.breaks)}replace(e,t,n){let s=n[0];return n.length==1&&(s instanceof Be||s instanceof de&&s.flags&4)&&Math.abs(this.length-s.length)<10?(s instanceof de?s=new Be(s.length,this.height,this.spaceAbove):s.height=this.height,this.outdated||(s.outdated=!1),s):Ae.of(n)}updateHeight(e,t=0,n=!1,s){return s&&s.from<=t&&s.more?this.setMeasuredHeight(s):(n||this.outdated)&&(this.spaceAbove=0,this.setHeight(Math.max(this.widgetHeight,e.heightForLine(this.length-this.collapsed))+this.breaks*e.lineHeight)),this.outdated=!1,this}toString(){return`line(${this.length}${this.collapsed?-this.collapsed:""}${this.widgetHeight?":"+this.widgetHeight:""})`}}class de extends Ae{constructor(e){super(e,0)}heightMetrics(e,t){let n=e.doc.lineAt(t).number,s=e.doc.lineAt(t+this.length).number,r=s-n+1,o,a=0;if(e.lineWrapping){let l=Math.min(this.height,e.lineHeight*r);o=l/r,this.length>r+1&&(a=(this.height-l)/(this.length-r-1))}else o=this.height/r;return{firstLine:n,lastLine:s,perLine:o,perChar:a}}blockAt(e,t,n,s){let{firstLine:r,lastLine:o,perLine:a,perChar:l}=this.heightMetrics(t,s);if(t.lineWrapping){let h=s+(e<t.lineHeight?0:Math.round(Math.max(0,Math.min(1,(e-n)/this.height))*this.length)),c=t.doc.lineAt(h),d=a+c.length*l,u=Math.max(n,e-d/2);return new Ke(c.from,c.length,u,d,0)}else{let h=Math.max(0,Math.min(o-r,Math.floor((e-n)/a))),{from:c,length:d}=t.doc.line(r+h);return new Ke(c,d,n+a*h,a,0)}}lineAt(e,t,n,s,r){if(t==X.ByHeight)return this.blockAt(e,n,s,r);if(t==X.ByPosNoHeight){let{from:f,to:p}=n.doc.lineAt(e);return new Ke(f,p-f,0,0,0)}let{firstLine:o,perLine:a,perChar:l}=this.heightMetrics(n,r),h=n.doc.lineAt(e),c=a+h.length*l,d=h.number-o,u=s+a*d+l*(h.from-r-d);return new Ke(h.from,h.length,Math.max(s,Math.min(u,s+this.height-c)),c,0)}forEachLine(e,t,n,s,r,o){e=Math.max(e,r),t=Math.min(t,r+this.length);let{firstLine:a,perLine:l,perChar:h}=this.heightMetrics(n,r);for(let c=e,d=s;c<=t;){let u=n.doc.lineAt(c);if(c==e){let p=u.number-a;d+=l*p+h*(e-r-p)}let f=l+h*u.length;o(new Ke(u.from,u.length,d,f,0)),d+=f,c=u.to+1}}replace(e,t,n){let s=this.length-t;if(s>0){let r=n[n.length-1];r instanceof de?n[n.length-1]=new de(r.length+s):n.push(null,new de(s-1))}if(e>0){let r=n[0];r instanceof de?n[0]=new de(e+r.length):n.unshift(new de(e-1),null)}return Ae.of(n)}decomposeLeft(e,t){t.push(new de(e-1),null)}decomposeRight(e,t){t.push(null,new de(this.length-e-1))}updateHeight(e,t=0,n=!1,s){let r=t+this.length;if(s&&s.from<=t+this.length&&s.more){let o=[],a=Math.max(t,s.from),l=-1;for(s.from>t&&o.push(new de(s.from-t-1).updateHeight(e,t));a<=r&&s.more;){let c=e.doc.lineAt(a).length;o.length&&o.push(null);let d=s.heights[s.index++],u=0;d<0&&(u=-d,d=s.heights[s.index++]),l==-1?l=d:Math.abs(d-l)>=Di&&(l=-2);let f=new Be(c,d,u);f.outdated=!1,o.push(f),a+=c+1}a<=r&&o.push(null,new de(r-a).updateHeight(e,a));let h=Ae.of(o);return(l<0||Math.abs(h.height-this.height)>=Di||Math.abs(l-this.heightMetrics(e,t).perLine)>=Di)&&(hn=!0),Ui(this,h)}else(n||this.outdated)&&(this.setHeight(e.heightForGap(t,t+this.length)),this.outdated=!1);return this}toString(){return`gap(${this.length})`}}class hf extends Ae{constructor(e,t,n){super(e.length+(t?1:0)+n.length,e.height+n.height,(t?1:0)|(e.outdated||n.outdated?2:0)),this.left=e,this.right=n,this.size=e.size+n.size}get break(){return this.flags&1}blockAt(e,t,n,s){let r=n+this.left.height;return e<r?this.left.blockAt(e,t,n,s):this.right.blockAt(e,t,r,s+this.left.length+this.break)}lineAt(e,t,n,s,r){let o=s+this.left.height,a=r+this.left.length+this.break,l=t==X.ByHeight?e<o:e<a,h=l?this.left.lineAt(e,t,n,s,r):this.right.lineAt(e,t,n,o,a);if(this.break||(l?h.to<a:h.from>a))return h;let c=t==X.ByPosNoHeight?X.ByPosNoHeight:X.ByPos;return l?h.join(this.right.lineAt(a,c,n,o,a)):this.left.lineAt(a,c,n,s,r).join(h)}forEachLine(e,t,n,s,r,o){let a=s+this.left.height,l=r+this.left.length+this.break;if(this.break)e<l&&this.left.forEachLine(e,t,n,s,r,o),t>=l&&this.right.forEachLine(e,t,n,a,l,o);else{let h=this.lineAt(l,X.ByPos,n,s,r);e<h.from&&this.left.forEachLine(e,Math.min(t,h.from-1),n,s,r,o),h.to>=e&&h.from<=t&&o(h),t>h.to&&this.right.forEachLine(Math.max(e,h.to+1),t,n,a,l,o)}}replace(e,t,n){let s=this.left.length+this.break;if(t<s)return this.balanced(this.left.replace(e,t,n),this.right);if(e>this.left.length)return this.balanced(this.left,this.right.replace(e-s,t-s,n));let r=[];e>0&&this.decomposeLeft(e,r);let o=r.length;for(let a of n)r.push(a);if(e>0&&ua(r,o-1),t<this.length){let a=r.length;this.decomposeRight(t,r),ua(r,a)}return Ae.of(r)}decomposeLeft(e,t){let n=this.left.length;if(e<=n)return this.left.decomposeLeft(e,t);t.push(this.left),this.break&&(n++,e>=n&&t.push(null)),e>n&&this.right.decomposeLeft(e-n,t)}decomposeRight(e,t){let n=this.left.length,s=n+this.break;if(e>=s)return this.right.decomposeRight(e-s,t);e<n&&this.left.decomposeRight(e,t),this.break&&e<s&&t.push(null),t.push(this.right)}balanced(e,t){return e.size>2*t.size||t.size>2*e.size?Ae.of(this.break?[e,null,t]:[e,t]):(this.left=Ui(this.left,e),this.right=Ui(this.right,t),this.setHeight(e.height+t.height),this.outdated=e.outdated||t.outdated,this.size=e.size+t.size,this.length=e.length+this.break+t.length,this)}updateHeight(e,t=0,n=!1,s){let{left:r,right:o}=this,a=t+r.length+this.break,l=null;return s&&s.from<=t+r.length&&s.more?l=r=r.updateHeight(e,t,n,s):r.updateHeight(e,t,n),s&&s.from<=a+o.length&&s.more?l=o=o.updateHeight(e,a,n,s):o.updateHeight(e,a,n),l?this.balanced(r,o):(this.height=this.left.height+this.right.height,this.outdated=!1,this)}toString(){return this.left+(this.break?" ":"-")+this.right}}function ua(i,e){let t,n;i[e]==null&&(t=i[e-1])instanceof de&&(n=i[e+1])instanceof de&&i.splice(e-1,3,new de(t.length+1+n.length))}const cf=5;class lo{constructor(e,t){this.pos=e,this.oracle=t,this.nodes=[],this.lineStart=-1,this.lineEnd=-1,this.covering=null,this.writtenTo=e}get isCovered(){return this.covering&&this.nodes[this.nodes.length-1]==this.covering}span(e,t){if(this.lineStart>-1){let n=Math.min(t,this.lineEnd),s=this.nodes[this.nodes.length-1];s instanceof Be?s.length+=n-this.pos:(n>this.pos||!this.isCovered)&&this.nodes.push(new Be(n-this.pos,-1,0)),this.writtenTo=n,t>n&&(this.nodes.push(null),this.writtenTo++,this.lineStart=-1)}this.pos=t}point(e,t,n){if(e<t||n.heightRelevant){let s=n.widget?n.widget.estimatedHeight:0,r=n.widget?n.widget.lineBreaks:0;s<0&&(s=this.oracle.lineHeight);let o=t-e;n.block?this.addBlock(new Sh(o,s,n)):(o||r||s>=cf)&&this.addLineDeco(s,r,o)}else t>e&&this.span(e,t);this.lineEnd>-1&&this.lineEnd<this.pos&&(this.lineEnd=this.oracle.doc.lineAt(this.pos).to)}enterLine(){if(this.lineStart>-1)return;let{from:e,to:t}=this.oracle.doc.lineAt(this.pos);this.lineStart=e,this.lineEnd=t,this.writtenTo<e&&((this.writtenTo<e-1||this.nodes[this.nodes.length-1]==null)&&this.nodes.push(this.blankContent(this.writtenTo,e-1)),this.nodes.push(null)),this.pos>e&&this.nodes.push(new Be(this.pos-e,-1,0)),this.writtenTo=this.pos}blankContent(e,t){let n=new de(t-e);return this.oracle.doc.lineAt(e).to==t&&(n.flags|=4),n}ensureLine(){this.enterLine();let e=this.nodes.length?this.nodes[this.nodes.length-1]:null;if(e instanceof Be)return e;let t=new Be(0,-1,0);return this.nodes.push(t),t}addBlock(e){this.enterLine();let t=e.deco;t&&t.startSide>0&&!this.isCovered&&this.ensureLine(),this.nodes.push(e),this.writtenTo=this.pos=this.pos+e.length,t&&t.endSide>0&&(this.covering=e)}addLineDeco(e,t,n){let s=this.ensureLine();s.length+=n,s.collapsed+=n,s.widgetHeight=Math.max(s.widgetHeight,e),s.breaks+=t,this.writtenTo=this.pos=this.pos+n}finish(e){let t=this.nodes.length==0?null:this.nodes[this.nodes.length-1];this.lineStart>-1&&!(t instanceof Be)&&!this.isCovered?this.nodes.push(new Be(0,-1,0)):(this.writtenTo<this.pos||t==null)&&this.nodes.push(this.blankContent(this.writtenTo,this.pos));let n=e;for(let s of this.nodes)s instanceof Be&&s.updateHeight(this.oracle,n),n+=s?s.length:1;return this.nodes}static build(e,t,n,s){let r=new lo(n,e);return H.spans(t,n,s,r,0),r.finish(n)}}function df(i,e,t){let n=new uf;return H.compare(i,e,t,n,0),n.changes}class uf{constructor(){this.changes=[]}compareRange(){}comparePoint(e,t,n,s){(e<t||n&&n.heightRelevant||s&&s.heightRelevant)&&Zt(e,t,this.changes,5)}}function ff(i,e){let t=i.getBoundingClientRect(),n=i.ownerDocument,s=n.defaultView||window,r=Math.max(0,t.left),o=Math.min(s.innerWidth,t.right),a=Math.max(0,t.top),l=Math.min(s.innerHeight,t.bottom);for(let h=i.parentNode;h&&h!=n.body;)if(h.nodeType==1){let c=h,d=window.getComputedStyle(c);if((c.scrollHeight>c.clientHeight||c.scrollWidth>c.clientWidth)&&d.overflow!="visible"){let u=c.getBoundingClientRect();r=Math.max(r,u.left),o=Math.min(o,u.right),a=Math.max(a,u.top),l=Math.min(h==i.parentNode?s.innerHeight:l,u.bottom)}h=d.position=="absolute"||d.position=="fixed"?c.offsetParent:c.parentNode}else if(h.nodeType==11)h=h.host;else break;return{left:r-t.left,right:Math.max(r,o)-t.left,top:a-(t.top+e),bottom:Math.max(a,l)-(t.top+e)}}function pf(i){let e=i.getBoundingClientRect(),t=i.ownerDocument.defaultView||window;return e.left<t.innerWidth&&e.right>0&&e.top<t.innerHeight&&e.bottom>0}function mf(i,e){let t=i.getBoundingClientRect();return{left:0,right:t.right-t.left,top:e,bottom:t.bottom-(t.top+e)}}class Vs{constructor(e,t,n,s){this.from=e,this.to=t,this.size=n,this.displaySize=s}static same(e,t){if(e.length!=t.length)return!1;for(let n=0;n<e.length;n++){let s=e[n],r=t[n];if(s.from!=r.from||s.to!=r.to||s.size!=r.size)return!1}return!0}draw(e,t){return _.replace({widget:new gf(this.displaySize*(t?e.scaleY:e.scaleX),t)}).range(this.from,this.to)}}class gf extends Xn{constructor(e,t){super(),this.size=e,this.vertical=t}eq(e){return e.size==this.size&&e.vertical==this.vertical}toDOM(){let e=document.createElement("div");return this.vertical?e.style.height=this.size+"px":(e.style.width=this.size+"px",e.style.height="2px",e.style.display="inline-block"),e}get estimatedHeight(){return this.vertical?this.size:-1}}class fa{constructor(e,t){this.view=e,this.state=t,this.pixelViewport={left:0,right:window.innerWidth,top:0,bottom:0},this.inView=!0,this.paddingTop=0,this.paddingBottom=0,this.contentDOMWidth=0,this.contentDOMHeight=0,this.editorHeight=0,this.editorWidth=0,this.scaleX=1,this.scaleY=1,this.scrollOffset=0,this.scrolledToBottom=!1,this.scrollAnchorPos=0,this.scrollAnchorHeight=-1,this.scaler=pa,this.scrollTarget=null,this.printing=!1,this.mustMeasureContent=!0,this.defaultTextDirection=J.LTR,this.visibleRanges=[],this.mustEnforceCursorAssoc=!1;let n=t.facet(so).some(s=>typeof s!="function"&&s.class=="cm-lineWrapping");this.heightOracle=new of(n),this.stateDeco=ma(t),this.heightMap=Ae.empty().applyChanges(this.stateDeco,j.empty,this.heightOracle.setDoc(t.doc),[new qe(0,0,0,t.doc.length)]);for(let s=0;s<2&&(this.viewport=this.getViewport(0,null),!!this.updateForViewport());s++);this.updateViewportLines(),this.lineGaps=this.ensureLineGaps([]),this.lineGapDeco=_.set(this.lineGaps.map(s=>s.draw(this,!1))),this.scrollParent=e.scrollDOM,this.computeVisibleRanges()}updateForViewport(){let e=[this.viewport],{main:t}=this.state.selection;for(let n=0;n<=1;n++){let s=n?t.head:t.anchor;if(!e.some(({from:r,to:o})=>s>=r&&s<=o)){let{from:r,to:o}=this.lineBlockAt(s);e.push(new hi(r,o))}}return this.viewports=e.sort((n,s)=>n.from-s.from),this.updateScaler()}updateScaler(){let e=this.scaler;return this.scaler=this.heightMap.height<=7e6?pa:new ho(this.heightOracle,this.heightMap,this.viewports),e.eq(this.scaler)?0:2}updateViewportLines(){this.viewportLines=[],this.heightMap.forEachLine(this.viewport.from,this.viewport.to,this.heightOracle.setDoc(this.state.doc),0,0,e=>{this.viewportLines.push(In(e,this.scaler))})}update(e,t=null){this.state=e.state;let n=this.stateDeco;this.stateDeco=ma(this.state);let s=e.changedRanges,r=qe.extendWithRanges(s,df(n,this.stateDeco,e?e.changes:oe.empty(this.state.doc.length))),o=this.heightMap.height,a=this.scrolledToBottom?null:this.scrollAnchorAt(this.scrollOffset);da(),this.heightMap=this.heightMap.applyChanges(this.stateDeco,e.startState.doc,this.heightOracle.setDoc(this.state.doc),r),(this.heightMap.height!=o||hn)&&(e.flags|=2),a?(this.scrollAnchorPos=e.changes.mapPos(a.from,-1),this.scrollAnchorHeight=a.top):(this.scrollAnchorPos=-1,this.scrollAnchorHeight=o);let l=r.length?this.mapViewport(this.viewport,e.changes):this.viewport;(t&&(t.range.head<l.from||t.range.head>l.to)||!this.viewportIsAppropriate(l))&&(l=this.getViewport(0,t));let h=l.from!=this.viewport.from||l.to!=this.viewport.to;this.viewport=l,e.flags|=this.updateForViewport(),(h||!e.changes.empty||e.flags&2)&&this.updateViewportLines(),(this.lineGaps.length||this.viewport.to-this.viewport.from>4e3)&&this.updateLineGaps(this.ensureLineGaps(this.mapLineGaps(this.lineGaps,e.changes))),e.flags|=this.computeVisibleRanges(e.changes),t&&(this.scrollTarget=t),!this.mustEnforceCursorAssoc&&(e.selectionSet||e.focusChanged)&&e.view.lineWrapping&&e.state.selection.main.empty&&e.state.selection.main.assoc&&!e.state.facet(nh)&&(this.mustEnforceCursorAssoc=!0)}measure(){let{view:e}=this,t=e.contentDOM,n=window.getComputedStyle(t),s=this.heightOracle,r=n.whiteSpace;this.defaultTextDirection=n.direction=="rtl"?J.RTL:J.LTR;let o=this.heightOracle.mustRefreshForWrapping(r)||this.mustMeasureContent==="refresh",a=t.getBoundingClientRect(),l=o||this.mustMeasureContent||this.contentDOMHeight!=a.height;this.contentDOMHeight=a.height,this.mustMeasureContent=!1;let h=0,c=0;if(a.width&&a.height){let{scaleX:S,scaleY:C}=Pl(t,a);(S>.005&&Math.abs(this.scaleX-S)>.005||C>.005&&Math.abs(this.scaleY-C)>.005)&&(this.scaleX=S,this.scaleY=C,h|=16,o=l=!0)}let d=(parseInt(n.paddingTop)||0)*this.scaleY,u=(parseInt(n.paddingBottom)||0)*this.scaleY;(this.paddingTop!=d||this.paddingBottom!=u)&&(this.paddingTop=d,this.paddingBottom=u,h|=18),this.editorWidth!=e.scrollDOM.clientWidth&&(s.lineWrapping&&(l=!0),this.editorWidth=e.scrollDOM.clientWidth,h|=16);let f=Nl(this.view.contentDOM,!1).y;f!=this.scrollParent&&(this.scrollParent=f,this.scrollAnchorHeight=-1,this.scrollOffset=0);let p=this.getScrollOffset();this.scrollOffset!=p&&(this.scrollAnchorHeight=-1,this.scrollOffset=p),this.scrolledToBottom=Fl(this.scrollParent||e.win);let m=(this.printing?mf:ff)(t,this.paddingTop),g=m.top-this.pixelViewport.top,y=m.bottom-this.pixelViewport.bottom;this.pixelViewport=m;let x=this.pixelViewport.bottom>this.pixelViewport.top&&this.pixelViewport.right>this.pixelViewport.left;if(x!=this.inView&&(this.inView=x,x&&(l=!0)),!this.inView&&!this.scrollTarget&&!pf(e.dom))return 0;let v=a.width;if((this.contentDOMWidth!=v||this.editorHeight!=e.scrollDOM.clientHeight)&&(this.contentDOMWidth=a.width,this.editorHeight=e.scrollDOM.clientHeight,h|=16),l){let S=e.docView.measureVisibleLineHeights(this.viewport);if(s.mustRefreshForHeights(S)&&(o=!0),o||s.lineWrapping&&Math.abs(v-this.contentDOMWidth)>s.charWidth){let{lineHeight:C,charWidth:A,textHeight:L}=e.docView.measureTextSize();o=C>0&&s.refresh(r,C,A,L,Math.max(5,v/A),S),o&&(e.docView.minWidth=0,h|=16)}g>0&&y>0?c=Math.max(g,y):g<0&&y<0&&(c=Math.min(g,y)),da();for(let C of this.viewports){let A=C.from==this.viewport.from?S:e.docView.measureVisibleLineHeights(C);this.heightMap=(o?Ae.empty().applyChanges(this.stateDeco,j.empty,this.heightOracle,[new qe(0,0,0,e.state.doc.length)]):this.heightMap).updateHeight(s,0,o,new af(C.from,A))}hn&&(h|=2)}let I=!this.viewportIsAppropriate(this.viewport,c)||this.scrollTarget&&(this.scrollTarget.range.head<this.viewport.from||this.scrollTarget.range.head>this.viewport.to);return I&&(h&2&&(h|=this.updateScaler()),this.viewport=this.getViewport(c,this.scrollTarget),h|=this.updateForViewport()),(h&2||I)&&this.updateViewportLines(),(this.lineGaps.length||this.viewport.to-this.viewport.from>4e3)&&this.updateLineGaps(this.ensureLineGaps(o?[]:this.lineGaps,e)),h|=this.computeVisibleRanges(),this.mustEnforceCursorAssoc&&(this.mustEnforceCursorAssoc=!1,e.docView.enforceCursorAssoc()),h}get visibleTop(){return this.scaler.fromDOM(this.pixelViewport.top)}get visibleBottom(){return this.scaler.fromDOM(this.pixelViewport.bottom)}getViewport(e,t){let n=.5-Math.max(-.5,Math.min(.5,e/1e3/2)),s=this.heightMap,r=this.heightOracle,{visibleTop:o,visibleBottom:a}=this,l=new hi(s.lineAt(o-n*1e3,X.ByHeight,r,0,0).from,s.lineAt(a+(1-n)*1e3,X.ByHeight,r,0,0).to);if(t){let{head:h}=t.range;if(h<l.from||h>l.to){let c=Math.min(this.editorHeight,this.pixelViewport.bottom-this.pixelViewport.top),d=s.lineAt(h,X.ByPos,r,0,0),u;t.y=="center"?u=(d.top+d.bottom)/2-c/2:t.y=="start"||t.y=="nearest"&&h<l.from?u=d.top:u=d.bottom-c,l=new hi(s.lineAt(u-1e3/2,X.ByHeight,r,0,0).from,s.lineAt(u+c+1e3/2,X.ByHeight,r,0,0).to)}}return l}mapViewport(e,t){let n=t.mapPos(e.from,-1),s=t.mapPos(e.to,1);return new hi(this.heightMap.lineAt(n,X.ByPos,this.heightOracle,0,0).from,this.heightMap.lineAt(s,X.ByPos,this.heightOracle,0,0).to)}viewportIsAppropriate({from:e,to:t},n=0){if(!this.inView)return!0;let{top:s}=this.heightMap.lineAt(e,X.ByPos,this.heightOracle,0,0),{bottom:r}=this.heightMap.lineAt(t,X.ByPos,this.heightOracle,0,0),{visibleTop:o,visibleBottom:a}=this;return(e==0||s<=o-Math.max(10,Math.min(-n,250)))&&(t==this.state.doc.length||r>=a+Math.max(10,Math.min(n,250)))&&s>o-2*1e3&&r<a+2*1e3}mapLineGaps(e,t){if(!e.length||t.empty)return e;let n=[];for(let s of e)t.touchesRange(s.from,s.to)||n.push(new Vs(t.mapPos(s.from),t.mapPos(s.to),s.size,s.displaySize));return n}ensureLineGaps(e,t){let n=this.heightOracle.lineWrapping,s=n?1e4:2e3,r=s>>1,o=s<<1;if(this.defaultTextDirection!=J.LTR&&!n)return[];let a=[],l=(c,d,u,f)=>{if(d-c<r)return;let p=this.state.selection.main,m=[p.from];p.empty||m.push(p.to);for(let y of m)if(y>c&&y<d){l(c,y-10,u,f),l(y+10,d,u,f);return}let g=bf(e,y=>y.from>=u.from&&y.to<=u.to&&Math.abs(y.from-c)<r&&Math.abs(y.to-d)<r&&!m.some(x=>y.from<x&&y.to>x));if(!g){if(d<u.to&&t&&n&&t.visibleRanges.some(v=>v.from<=d&&v.to>=d)){let v=t.moveToLineBoundary(b.cursor(d),!1,!0).head;v>c&&(d=v)}let y=this.gapSize(u,c,d,f),x=n||y<2e6?y:2e6;g=new Vs(c,d,y,x)}a.push(g)},h=c=>{if(c.length<o||c.type!=pe.Text)return;let d=yf(c.from,c.to,this.stateDeco);if(d.total<o)return;let u=this.scrollTarget?this.scrollTarget.range.head:null,f,p;if(n){let m=s/this.heightOracle.lineLength*this.heightOracle.lineHeight,g,y;if(u!=null){let x=di(d,u),v=((this.visibleBottom-this.visibleTop)/2+m)/c.height;g=x-v,y=x+v}else g=(this.visibleTop-c.top-m)/c.height,y=(this.visibleBottom-c.top+m)/c.height;f=ci(d,g),p=ci(d,y)}else{let m=d.total*this.heightOracle.charWidth,g=s*this.heightOracle.charWidth,y=0;if(m>2e6)for(let C of e)C.from>=c.from&&C.from<c.to&&C.size!=C.displaySize&&C.from*this.heightOracle.charWidth+y<this.pixelViewport.left&&(y=C.size-C.displaySize);let x=this.pixelViewport.left+y,v=this.pixelViewport.right+y,I,S;if(u!=null){let C=di(d,u),A=((v-x)/2+g)/m;I=C-A,S=C+A}else I=(x-g)/m,S=(v+g)/m;f=ci(d,I),p=ci(d,S)}f>c.from&&l(c.from,f,c,d),p<c.to&&l(p,c.to,c,d)};for(let c of this.viewportLines)Array.isArray(c.type)?c.type.forEach(h):h(c);return a}gapSize(e,t,n,s){let r=di(s,n)-di(s,t);return this.heightOracle.lineWrapping?e.height*r:s.total*this.heightOracle.charWidth*r}updateLineGaps(e){Vs.same(e,this.lineGaps)||(this.lineGaps=e,this.lineGapDeco=_.set(e.map(t=>t.draw(this,this.heightOracle.lineWrapping))))}computeVisibleRanges(e){let t=this.stateDeco;this.lineGaps.length&&(t=t.concat(this.lineGapDeco));let n=[];H.spans(t,this.viewport.from,this.viewport.to,{span(r,o){n.push({from:r,to:o})},point(){}},20);let s=0;if(n.length!=this.visibleRanges.length)s=12;else for(let r=0;r<n.length&&!(s&8);r++){let o=this.visibleRanges[r],a=n[r];(o.from!=a.from||o.to!=a.to)&&(s|=4,e&&e.mapPos(o.from,-1)==a.from&&e.mapPos(o.to,1)==a.to||(s|=8))}return this.visibleRanges=n,s}lineBlockAt(e){return e>=this.viewport.from&&e<=this.viewport.to&&this.viewportLines.find(t=>t.from<=e&&t.to>=e)||In(this.heightMap.lineAt(e,X.ByPos,this.heightOracle,0,0),this.scaler)}lineBlockAtHeight(e){return e>=this.viewportLines[0].top&&e<=this.viewportLines[this.viewportLines.length-1].bottom&&this.viewportLines.find(t=>t.top<=e&&t.bottom>=e)||In(this.heightMap.lineAt(this.scaler.fromDOM(e),X.ByHeight,this.heightOracle,0,0),this.scaler)}getScrollOffset(){return this.scrollParent==this.view.scrollDOM?this.scrollParent.scrollTop*this.scaleY:(this.scrollParent?this.scrollParent.getBoundingClientRect().top:0)-this.view.contentDOM.getBoundingClientRect().top}scrollAnchorAt(e){let t=this.lineBlockAtHeight(e+8);return t.from>=this.viewport.from||this.viewportLines[0].top-e>200?t:this.viewportLines[0]}elementAtHeight(e){return In(this.heightMap.blockAt(this.scaler.fromDOM(e),this.heightOracle,0,0),this.scaler)}get docHeight(){return this.scaler.toDOM(this.heightMap.height)}get contentHeight(){return this.docHeight+this.paddingTop+this.paddingBottom}}class hi{constructor(e,t){this.from=e,this.to=t}}function yf(i,e,t){let n=[],s=i,r=0;return H.spans(t,i,e,{span(){},point(o,a){o>s&&(n.push({from:s,to:o}),r+=o-s),s=a}},20),s<e&&(n.push({from:s,to:e}),r+=e-s),{total:r,ranges:n}}function ci({total:i,ranges:e},t){if(t<=0)return e[0].from;if(t>=1)return e[e.length-1].to;let n=Math.floor(i*t);for(let s=0;;s++){let{from:r,to:o}=e[s],a=o-r;if(n<=a)return r+n;n-=a}}function di(i,e){let t=0;for(let{from:n,to:s}of i.ranges){if(e<=s){t+=e-n;break}t+=s-n}return t/i.total}function bf(i,e){for(let t of i)if(e(t))return t}const pa={toDOM(i){return i},fromDOM(i){return i},scale:1,eq(i){return i==this}};function ma(i){let e=i.facet(ds).filter(n=>typeof n!="function"),t=i.facet(ro).filter(n=>typeof n!="function");return t.length&&e.push(H.join(t)),e}class ho{constructor(e,t,n){let s=0,r=0,o=0;this.viewports=n.map(({from:a,to:l})=>{let h=t.lineAt(a,X.ByPos,e,0,0).top,c=t.lineAt(l,X.ByPos,e,0,0).bottom;return s+=c-h,{from:a,to:l,top:h,bottom:c,domTop:0,domBottom:0}}),this.scale=(7e6-s)/(t.height-s);for(let a of this.viewports)a.domTop=o+(a.top-r)*this.scale,o=a.domBottom=a.domTop+(a.bottom-a.top),r=a.bottom}toDOM(e){for(let t=0,n=0,s=0;;t++){let r=t<this.viewports.length?this.viewports[t]:null;if(!r||e<r.top)return s+(e-n)*this.scale;if(e<=r.bottom)return r.domTop+(e-r.top);n=r.bottom,s=r.domBottom}}fromDOM(e){for(let t=0,n=0,s=0;;t++){let r=t<this.viewports.length?this.viewports[t]:null;if(!r||e<r.domTop)return n+(e-s)/this.scale;if(e<=r.domBottom)return r.top+(e-r.domTop);n=r.bottom,s=r.domBottom}}eq(e){return e instanceof ho?this.scale==e.scale&&this.viewports.length==e.viewports.length&&this.viewports.every((t,n)=>t.from==e.viewports[n].from&&t.to==e.viewports[n].to):!1}}function In(i,e){if(e.scale==1)return i;let t=e.toDOM(i.top),n=e.toDOM(i.bottom);return new Ke(i.from,i.length,t,n-t,Array.isArray(i._content)?i._content.map(s=>In(s,e)):i._content)}const ui=D.define({combine:i=>i.join(" ")}),Er=D.define({combine:i=>i.indexOf(!0)>-1}),Lr=on.newName(),Ch=on.newName(),Ah=on.newName(),Th={"&light":"."+Ch,"&dark":"."+Ah};function Br(i,e,t){return new on(e,{finish(n){return/&/.test(n)?n.replace(/&\w*/,s=>{if(s=="&")return i;if(!t||!t[s])throw new RangeError(`Unsupported selector: ${s}`);return t[s]}):i+" "+n}})}const wf=Br("."+Lr,{"&":{position:"relative !important",boxSizing:"border-box","&.cm-focused":{outline:"1px dotted #212121"},display:"flex !important",flexDirection:"column"},".cm-scroller":{display:"flex !important",alignItems:"flex-start !important",fontFamily:"monospace",lineHeight:1.4,height:"100%",overflowX:"auto",position:"relative",zIndex:0,overflowAnchor:"none"},".cm-content":{margin:0,flexGrow:2,flexShrink:0,display:"block",whiteSpace:"pre",wordWrap:"normal",boxSizing:"border-box",minHeight:"100%",padding:"4px 0",outline:"none","&[contenteditable=true]":{WebkitUserModify:"read-write-plaintext-only"}},".cm-lineWrapping":{whiteSpace_fallback:"pre-wrap",whiteSpace:"break-spaces",wordBreak:"break-word",overflowWrap:"anywhere",flexShrink:1},"&light .cm-content":{caretColor:"black"},"&dark .cm-content":{caretColor:"white"},".cm-line":{display:"block",padding:"0 2px 0 6px"},".cm-layer":{userSelect:"none",position:"absolute",left:0,top:0,contain:"size style","& > *":{position:"absolute"}},"&light .cm-selectionBackground":{background:"#d9d9d9"},"&dark .cm-selectionBackground":{background:"#222"},"&light.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground":{background:"#d7d4f0"},"&dark.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground":{background:"#233"},".cm-cursorLayer":{pointerEvents:"none"},"&.cm-focused > .cm-scroller > .cm-cursorLayer":{animation:"steps(1) cm-blink 1.2s infinite"},"@keyframes cm-blink":{"0%":{},"50%":{opacity:0},"100%":{}},"@keyframes cm-blink2":{"0%":{},"50%":{opacity:0},"100%":{}},".cm-cursor, .cm-dropCursor":{borderLeft:"1.2px solid black",marginLeft:"-0.6px",pointerEvents:"none"},".cm-cursor":{display:"none"},"&dark .cm-cursor":{borderLeftColor:"#ddd"},".cm-selectionHandle":{backgroundColor:"currentColor",width:"1.5px"},".cm-selectionHandle-start::before, .cm-selectionHandle-end::before":{content:'""',backgroundColor:"inherit",borderRadius:"50%",width:"8px",height:"8px",position:"absolute",left:"-3.25px"},".cm-selectionHandle-start::before":{top:"-8px"},".cm-selectionHandle-end::before":{bottom:"-8px"},".cm-dropCursor":{position:"absolute"},"&.cm-focused > .cm-scroller > .cm-cursorLayer .cm-cursor":{display:"block"},".cm-iso":{unicodeBidi:"isolate"},".cm-announced":{position:"fixed",top:"-10000px"},"@media print":{".cm-announced":{display:"none"}},"&light .cm-activeLine":{backgroundColor:"#cceeff44"},"&dark .cm-activeLine":{backgroundColor:"#99eeff33"},"&light .cm-specialChar":{color:"red"},"&dark .cm-specialChar":{color:"#f78"},".cm-gutters":{flexShrink:0,display:"flex",height:"100%",boxSizing:"border-box",zIndex:200},".cm-gutters-before":{insetInlineStart:0},".cm-gutters-after":{insetInlineEnd:0},"&light .cm-gutters":{backgroundColor:"#f5f5f5",color:"#6c6c6c",border:"0px solid #ddd","&.cm-gutters-before":{borderRightWidth:"1px"},"&.cm-gutters-after":{borderLeftWidth:"1px"}},"&dark .cm-gutters":{backgroundColor:"#333338",color:"#ccc"},".cm-gutter":{display:"flex !important",flexDirection:"column",flexShrink:0,boxSizing:"border-box",minHeight:"100%",overflow:"hidden"},".cm-gutterElement":{boxSizing:"border-box"},".cm-lineNumbers .cm-gutterElement":{padding:"0 3px 0 5px",minWidth:"20px",textAlign:"right",whiteSpace:"nowrap"},"&light .cm-activeLineGutter":{backgroundColor:"#e2f2ff"},"&dark .cm-activeLineGutter":{backgroundColor:"#222227"},".cm-panels":{boxSizing:"border-box",position:"sticky",left:0,right:0,zIndex:300},"&light .cm-panels":{backgroundColor:"#f5f5f5",color:"black"},".cm-panels-top":{top:"0"},".cm-panels-bottom":{bottom:"0"},"&light .cm-panels-top":{borderBottom:"1px solid #ddd"},"&light .cm-panels-bottom":{borderTop:"1px solid #ddd"},"&dark .cm-panels":{backgroundColor:"#333338",color:"white"},".cm-dialog":{padding:"2px 19px 4px 6px",position:"relative","& label":{fontSize:"80%"}},".cm-dialog-close":{position:"absolute",top:"3px",right:"4px",backgroundColor:"inherit",border:"none",font:"inherit",fontSize:"14px",padding:"0"},".cm-tab":{display:"inline-block",overflow:"hidden",verticalAlign:"bottom"},".cm-widgetBuffer":{verticalAlign:"text-top",height:"1em",width:0,display:"inline"},".cm-placeholder":{color:"#888",display:"inline-block",verticalAlign:"top",userSelect:"none"},".cm-highlightSpace":{background:"radial-gradient(circle at 50% 55%, #aaa 20%, transparent 0) no-repeat",backgroundSize:".4em",backgroundPosition:"calc(min(50%, 0px)) center"},".cm-highlightTab":{backgroundImage:`url('data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" width="200" height="20"><path stroke="%23888" stroke-width="1" fill="none" d="M1 10H196L190 5M190 15L196 10M197 4L197 16"/></svg>')`,backgroundSize:"auto 100%",backgroundPosition:"right 90%",backgroundRepeat:"no-repeat"},".cm-trailingSpace":{backgroundColor:"#ff332255"},".cm-button":{verticalAlign:"middle",color:"inherit",fontSize:"70%",padding:".2em 1em",borderRadius:"1px"},"&light .cm-button":{backgroundImage:"linear-gradient(#eff1f5, #d9d9df)",border:"1px solid #888","&:active":{backgroundImage:"linear-gradient(#b4b4b4, #d0d3d6)"}},"&dark .cm-button":{backgroundImage:"linear-gradient(#393939, #111)",border:"1px solid #888","&:active":{backgroundImage:"linear-gradient(#111, #333)"}},".cm-textfield":{verticalAlign:"middle",color:"inherit",fontSize:"70%",border:"1px solid silver",padding:".2em .5em"},"&light .cm-textfield":{backgroundColor:"white"},"&dark .cm-textfield":{border:"1px solid #555",backgroundColor:"inherit"}},Th),vf={childList:!0,characterData:!0,subtree:!0,attributes:!0,characterDataOldValue:!0},zs=O.ie&&O.ie_version<=11;class kf{constructor(e){this.view=e,this.active=!1,this.editContext=null,this.selectionRange=new Yd,this.selectionChanged=!1,this.delayedFlush=-1,this.resizeTimeout=-1,this.queue=[],this.delayedAndroidKey=null,this.flushingAndroidKey=-1,this.lastChange=0,this.scrollTargets=[],this.intersection=null,this.resizeScroll=null,this.intersecting=!1,this.gapIntersection=null,this.gaps=[],this.printQuery=null,this.parentCheck=-1,this.dom=e.contentDOM,this.observer=new MutationObserver(t=>{for(let n of t)this.queue.push(n);(O.ie&&O.ie_version<=11||O.ios&&e.composing)&&t.some(n=>n.type=="childList"&&n.removedNodes.length||n.type=="characterData"&&n.oldValue.length>n.target.nodeValue.length)?this.flushSoon():this.flush()}),window.EditContext&&O.android&&e.constructor.EDIT_CONTEXT!==!1&&!(O.chrome&&O.chrome_version<126)&&(this.editContext=new Sf(e),e.state.facet(ut)&&(e.contentDOM.editContext=this.editContext.editContext)),zs&&(this.onCharData=t=>{this.queue.push({target:t.target,type:"characterData",oldValue:t.prevValue}),this.flushSoon()}),this.onSelectionChange=this.onSelectionChange.bind(this),this.onResize=this.onResize.bind(this),this.onPrint=this.onPrint.bind(this),this.onScroll=this.onScroll.bind(this),window.matchMedia&&(this.printQuery=window.matchMedia("print")),typeof ResizeObserver=="function"&&(this.resizeScroll=new ResizeObserver(()=>{var t;((t=this.view.docView)===null||t===void 0?void 0:t.lastUpdate)<Date.now()-75&&this.onResize()}),this.resizeScroll.observe(e.scrollDOM)),this.addWindowListeners(this.win=e.win),this.start(),typeof IntersectionObserver=="function"&&(this.intersection=new IntersectionObserver(t=>{this.parentCheck<0&&(this.parentCheck=setTimeout(this.listenForScroll.bind(this),1e3)),t.length>0&&t[t.length-1].intersectionRatio>0!=this.intersecting&&(this.intersecting=!this.intersecting,this.intersecting!=this.view.inView&&this.onScrollChanged(document.createEvent("Event")))},{threshold:[0,.001]}),this.intersection.observe(this.dom),this.gapIntersection=new IntersectionObserver(t=>{t.length>0&&t[t.length-1].intersectionRatio>0&&this.onScrollChanged(document.createEvent("Event"))},{})),this.listenForScroll(),this.readSelectionRange()}onScrollChanged(e){this.view.inputState.runHandlers("scroll",e),this.intersecting&&this.view.measure()}onScroll(e){this.intersecting&&this.flush(!1),this.editContext&&this.view.requestMeasure(this.editContext.measureReq),this.onScrollChanged(e)}onResize(){this.resizeTimeout<0&&(this.resizeTimeout=setTimeout(()=>{this.resizeTimeout=-1,this.view.requestMeasure()},50))}onPrint(e){(e.type=="change"||!e.type)&&!e.matches||(this.view.viewState.printing=!0,this.view.measure(),setTimeout(()=>{this.view.viewState.printing=!1,this.view.requestMeasure()},500))}updateGaps(e){if(this.gapIntersection&&(e.length!=this.gaps.length||this.gaps.some((t,n)=>t!=e[n]))){this.gapIntersection.disconnect();for(let t of e)this.gapIntersection.observe(t);this.gaps=e}}onSelectionChange(e){let t=this.selectionChanged;if(!this.readSelectionRange()||this.delayedAndroidKey)return;let{view:n}=this,s=this.selectionRange;if(n.state.facet(ut)?n.root.activeElement!=this.dom:!En(this.dom,s))return;let r=s.anchorNode&&n.docView.tile.nearest(s.anchorNode);if(r&&r.isWidget()&&r.widget.ignoreEvent(e)){t||(this.selectionChanged=!1);return}(O.ie&&O.ie_version<=11||O.android&&O.chrome)&&!n.state.selection.main.empty&&s.focusNode&&Ln(s.focusNode,s.focusOffset,s.anchorNode,s.anchorOffset)?this.flushSoon():this.flush(!1)}readSelectionRange(){let{view:e}=this,t=zn(e.root);if(!t)return!1;let n=O.safari&&e.root.nodeType==11&&e.root.activeElement==this.dom&&xf(this.view,t)||t;if(!n||this.selectionRange.eq(n))return!1;let s=En(this.dom,n);return s&&!this.selectionChanged&&e.inputState.lastFocusTime>Date.now()-200&&e.inputState.lastTouchTime<Date.now()-300&&Xd(this.dom,n)?(this.view.inputState.lastFocusTime=0,e.docView.updateSelection(),!1):(this.selectionRange.setRange(n),s&&(this.selectionChanged=!0),!0)}setSelectionRange(e,t){this.selectionRange.set(e.node,e.offset,t.node,t.offset),this.selectionChanged=!1}clearSelectionRange(){this.selectionRange.set(null,0,null,0)}listenForScroll(){this.parentCheck=-1;let e=0,t=null;for(let n=this.dom;n;)if(n.nodeType==1)!t&&e<this.scrollTargets.length&&this.scrollTargets[e]==n?e++:t||(t=this.scrollTargets.slice(0,e)),t&&t.push(n),n=n.assignedSlot||n.parentNode;else if(n.nodeType==11)n=n.host;else break;if(e<this.scrollTargets.length&&!t&&(t=this.scrollTargets.slice(0,e)),t){for(let n of this.scrollTargets)n.removeEventListener("scroll",this.onScroll);for(let n of this.scrollTargets=t)n.addEventListener("scroll",this.onScroll)}}ignore(e){if(!this.active)return e();try{return this.stop(),e()}finally{this.start(),this.clear()}}start(){this.active||(this.observer.observe(this.dom,vf),zs&&this.dom.addEventListener("DOMCharacterDataModified",this.onCharData),this.active=!0)}stop(){this.active&&(this.active=!1,this.observer.disconnect(),zs&&this.dom.removeEventListener("DOMCharacterDataModified",this.onCharData))}clear(){this.processRecords(),this.queue.length=0,this.selectionChanged=!1}delayAndroidKey(e,t){var n;if(!this.delayedAndroidKey){let s=()=>{let r=this.delayedAndroidKey;r&&(this.clearDelayedAndroidKey(),this.view.inputState.lastKeyCode=r.keyCode,this.view.inputState.lastKeyTime=Date.now(),!this.flush()&&r.force&&en(this.dom,r.key,r.keyCode))};this.flushingAndroidKey=this.view.win.requestAnimationFrame(s)}(!this.delayedAndroidKey||e=="Enter")&&(this.delayedAndroidKey={key:e,keyCode:t,force:this.lastChange<Date.now()-50||!!(!((n=this.delayedAndroidKey)===null||n===void 0)&&n.force)})}clearDelayedAndroidKey(){this.win.cancelAnimationFrame(this.flushingAndroidKey),this.delayedAndroidKey=null,this.flushingAndroidKey=-1}flushSoon(){this.delayedFlush<0&&(this.delayedFlush=this.view.win.requestAnimationFrame(()=>{this.delayedFlush=-1,this.flush()}))}forceFlush(){this.delayedFlush>=0&&(this.view.win.cancelAnimationFrame(this.delayedFlush),this.delayedFlush=-1),this.flush()}pendingRecords(){for(let e of this.observer.takeRecords())this.queue.push(e);return this.queue}processRecords(){let e=this.pendingRecords();e.length&&(this.queue=[]);let t=-1,n=-1,s=!1;for(let r of e){let o=this.readMutation(r);o&&(o.typeOver&&(s=!0),t==-1?{from:t,to:n}=o:(t=Math.min(o.from,t),n=Math.max(o.to,n)))}return{from:t,to:n,typeOver:s}}readChange(){let{from:e,to:t,typeOver:n}=this.processRecords(),s=this.selectionChanged&&En(this.dom,this.selectionRange);if(e<0&&!s)return null;e>-1&&(this.lastChange=Date.now()),this.view.inputState.lastFocusTime=0,this.selectionChanged=!1;let r=new Wu(this.view,e,t,n);return this.view.docView.domChanged={newSel:r.newSel?r.newSel.main:null},r}flush(e=!0){if(this.delayedFlush>=0||this.delayedAndroidKey)return!1;e&&this.readSelectionRange();let t=this.readChange();if(!t)return this.view.requestMeasure(),!1;let n=this.view.state,s=fh(this.view,t);return this.view.state==n&&(t.domChanged||t.newSel&&!ji(this.view.state.selection,t.newSel.main))&&this.view.update([]),s}readMutation(e){let t=this.view.docView.tile.nearest(e.target);if(!t||t.isWidget())return null;if(t.markDirty(e.type=="attributes"),e.type=="childList"){let n=ga(t,e.previousSibling||e.target.previousSibling,-1),s=ga(t,e.nextSibling||e.target.nextSibling,1);return{from:n?t.posAfter(n):t.posAtStart,to:s?t.posBefore(s):t.posAtEnd,typeOver:!1}}else return e.type=="characterData"?{from:t.posAtStart,to:t.posAtEnd,typeOver:e.target.nodeValue==e.oldValue}:null}setWindow(e){e!=this.win&&(this.removeWindowListeners(this.win),this.win=e,this.addWindowListeners(this.win))}addWindowListeners(e){e.addEventListener("resize",this.onResize),this.printQuery?this.printQuery.addEventListener?this.printQuery.addEventListener("change",this.onPrint):this.printQuery.addListener(this.onPrint):e.addEventListener("beforeprint",this.onPrint),e.addEventListener("scroll",this.onScroll),e.document.addEventListener("selectionchange",this.onSelectionChange)}removeWindowListeners(e){e.removeEventListener("scroll",this.onScroll),e.removeEventListener("resize",this.onResize),this.printQuery?this.printQuery.removeEventListener?this.printQuery.removeEventListener("change",this.onPrint):this.printQuery.removeListener(this.onPrint):e.removeEventListener("beforeprint",this.onPrint),e.document.removeEventListener("selectionchange",this.onSelectionChange)}update(e){this.editContext&&(this.editContext.update(e),e.startState.facet(ut)!=e.state.facet(ut)&&(e.view.contentDOM.editContext=e.state.facet(ut)?this.editContext.editContext:null))}destroy(){var e,t,n;this.stop(),(e=this.intersection)===null||e===void 0||e.disconnect(),(t=this.gapIntersection)===null||t===void 0||t.disconnect(),(n=this.resizeScroll)===null||n===void 0||n.disconnect();for(let s of this.scrollTargets)s.removeEventListener("scroll",this.onScroll);this.removeWindowListeners(this.win),clearTimeout(this.parentCheck),clearTimeout(this.resizeTimeout),this.win.cancelAnimationFrame(this.delayedFlush),this.win.cancelAnimationFrame(this.flushingAndroidKey),this.editContext&&(this.view.contentDOM.editContext=null,this.editContext.destroy())}}function ga(i,e,t){for(;e;){let n=te.get(e);if(n&&n.parent==i)return n;let s=e.parentNode;e=s!=i.dom?s:t>0?e.nextSibling:e.previousSibling}return null}function ya(i,e){let t=e.startContainer,n=e.startOffset,s=e.endContainer,r=e.endOffset,o=i.docView.domAtPos(i.state.selection.main.anchor,1);return Ln(o.node,o.offset,s,r)&&([t,n,s,r]=[s,r,t,n]),{anchorNode:t,anchorOffset:n,focusNode:s,focusOffset:r}}function xf(i,e){if(e.getComposedRanges){let s=e.getComposedRanges(i.root)[0];if(s)return ya(i,s)}let t=null;function n(s){s.preventDefault(),s.stopImmediatePropagation(),t=s.getTargetRanges()[0]}return i.contentDOM.addEventListener("beforeinput",n,!0),i.dom.ownerDocument.execCommand("indent"),i.contentDOM.removeEventListener("beforeinput",n,!0),t?ya(i,t):null}class Sf{constructor(e){this.from=0,this.to=0,this.pendingContextChange=null,this.handlers=Object.create(null),this.composing=null,this.resetRange(e.state);let t=this.editContext=new window.EditContext({text:e.state.doc.sliceString(this.from,this.to),selectionStart:this.toContextPos(Math.max(this.from,Math.min(this.to,e.state.selection.main.anchor))),selectionEnd:this.toContextPos(e.state.selection.main.head)});this.handlers.textupdate=n=>{let s=e.state.selection.main,{anchor:r,head:o}=s,a=this.toEditorPos(n.updateRangeStart),l=this.toEditorPos(n.updateRangeEnd);e.inputState.composing>=0&&!this.composing&&(this.composing={contextBase:n.updateRangeStart,editorBase:a,drifted:!1});let h=l-a>n.text.length;a==this.from&&r<this.from?a=r:l==this.to&&r>this.to&&(l=r);let c=ph(e.state.sliceDoc(a,l),n.text,(h?s.from:s.to)-a,h?"end":null);if(!c){let u=b.single(this.toEditorPos(n.selectionStart),this.toEditorPos(n.selectionEnd));ji(u,s)||e.dispatch({selection:u,userEvent:"select"});return}let d={from:c.from+a,to:c.toA+a,insert:j.of(n.text.slice(c.from,c.toB).split(`
`))};if((O.mac||O.android)&&d.from==o-1&&/^\. ?$/.test(n.text)&&e.contentDOM.getAttribute("autocorrect")=="off"&&(d={from:a,to:l,insert:j.of([n.text.replace("."," ")])}),this.pendingContextChange=d,!e.state.readOnly){let u=this.to-this.from+(d.to-d.from+d.insert.length);ao(e,d,b.single(this.toEditorPos(n.selectionStart,u),this.toEditorPos(n.selectionEnd,u)))}this.pendingContextChange&&(this.revertPending(e.state),this.setSelection(e.state)),d.from<d.to&&!d.insert.length&&e.inputState.composing>=0&&!/[\\p{Alphabetic}\\p{Number}_]/.test(t.text.slice(Math.max(0,n.updateRangeStart-1),Math.min(t.text.length,n.updateRangeStart+1)))&&this.handlers.compositionend(n)},this.handlers.characterboundsupdate=n=>{let s=[],r=null;for(let o=this.toEditorPos(n.rangeStart),a=this.toEditorPos(n.rangeEnd);o<a;o++){let l=e.coordsForChar(o);r=l&&new DOMRect(l.left,l.top,l.right-l.left,l.bottom-l.top)||r||new DOMRect,s.push(r)}t.updateCharacterBounds(n.rangeStart,s)},this.handlers.textformatupdate=n=>{let s=[];for(let r of n.getTextFormats()){let o=r.underlineStyle,a=r.underlineThickness;if(!/none/i.test(o)&&!/none/i.test(a)){let l=this.toEditorPos(r.rangeStart),h=this.toEditorPos(r.rangeEnd);if(l<h){let c=`text-decoration: underline ${/^[a-z]/.test(o)?o+" ":o=="Dashed"?"dashed ":o=="Squiggle"?"wavy ":""}${/thin/i.test(a)?1:2}px`;s.push(_.mark({attributes:{style:c}}).range(l,h))}}}e.dispatch({effects:sh.of(_.set(s))})},this.handlers.compositionstart=()=>{e.inputState.composing<0&&(e.inputState.composing=0,e.inputState.compositionFirstChange=!0)},this.handlers.compositionend=()=>{if(e.inputState.composing=-1,e.inputState.compositionFirstChange=null,this.composing){let{drifted:n}=this.composing;this.composing=null,n&&this.reset(e.state)}};for(let n in this.handlers)t.addEventListener(n,this.handlers[n]);this.measureReq={read:n=>{let s=zn(n.root);s&&s.rangeCount&&this.editContext.updateSelectionBounds(s.getRangeAt(0).getBoundingClientRect())}}}applyEdits(e){let t=0,n=!1,s=this.pendingContextChange;return e.changes.iterChanges((r,o,a,l,h)=>{if(n)return;let c=h.length-(o-r);if(s&&o>=s.to)if(s.from==r&&s.to==o&&s.insert.eq(h)){s=this.pendingContextChange=null,t+=c,this.to+=c;return}else s=null,this.revertPending(e.state);if(r+=t,o+=t,o<=this.from)this.from+=c,this.to+=c;else if(r<this.to){if(r<this.from||o>this.to||this.to-this.from+h.length>3e4){n=!0;return}this.editContext.updateText(this.toContextPos(r),this.toContextPos(o),h.toString()),this.to+=c}t+=c}),s&&!n&&this.revertPending(e.state),!n}update(e){let t=this.pendingContextChange,n=e.startState.selection.main;this.composing&&(this.composing.drifted||!e.changes.touchesRange(n.from,n.to)&&e.transactions.some(s=>!s.isUserEvent("input.type")&&s.changes.touchesRange(this.from,this.to)))?(this.composing.drifted=!0,this.composing.editorBase=e.changes.mapPos(this.composing.editorBase)):!this.applyEdits(e)||!this.rangeIsValid(e.state)?(this.pendingContextChange=null,this.reset(e.state)):(e.docChanged||e.selectionSet||t)&&this.setSelection(e.state),(e.geometryChanged||e.docChanged||e.selectionSet)&&e.view.requestMeasure(this.measureReq)}resetRange(e){let{head:t}=e.selection.main;this.from=Math.max(0,t-1e4),this.to=Math.min(e.doc.length,t+1e4)}reset(e){this.resetRange(e),this.editContext.updateText(0,this.editContext.text.length,e.doc.sliceString(this.from,this.to)),this.setSelection(e)}revertPending(e){let t=this.pendingContextChange;this.pendingContextChange=null,this.editContext.updateText(this.toContextPos(t.from),this.toContextPos(t.from+t.insert.length),e.doc.sliceString(t.from,t.to))}setSelection(e){let{main:t}=e.selection,n=this.toContextPos(Math.max(this.from,Math.min(this.to,t.anchor))),s=this.toContextPos(t.head);(this.editContext.selectionStart!=n||this.editContext.selectionEnd!=s)&&this.editContext.updateSelection(n,s)}rangeIsValid(e){let{head:t}=e.selection.main;return!(this.from>0&&t-this.from<500||this.to<e.doc.length&&this.to-t<500||this.to-this.from>1e4*3)}toEditorPos(e,t=this.to-this.from){e=Math.min(e,t);let n=this.composing;return n&&n.drifted?n.editorBase+(e-n.contextBase):e+this.from}toContextPos(e){let t=this.composing;return t&&t.drifted?t.contextBase+(e-t.editorBase):e-this.from}destroy(){for(let e in this.handlers)this.editContext.removeEventListener(e,this.handlers[e])}}class R{get state(){return this.viewState.state}get viewport(){return this.viewState.viewport}get visibleRanges(){return this.viewState.visibleRanges}get inView(){return this.viewState.inView}get composing(){return!!this.inputState&&this.inputState.composing>0}get compositionStarted(){return!!this.inputState&&this.inputState.composing>=0}get root(){return this._root}get win(){return this.dom.ownerDocument.defaultView||window}constructor(e={}){var t;this.plugins=[],this.pluginMap=new Map,this.editorAttrs={},this.contentAttrs={},this.bidiCache=[],this.destroyed=!1,this.updateState=2,this.measureScheduled=-1,this.measureRequests=[],this.clearAnnouncement=-1,this.contentDOM=document.createElement("div"),this.scrollDOM=document.createElement("div"),this.scrollDOM.tabIndex=-1,this.scrollDOM.className="cm-scroller",this.scrollDOM.appendChild(this.contentDOM),this.announceDOM=document.createElement("div"),this.announceDOM.className="cm-announced",this.announceDOM.setAttribute("aria-live","polite"),this.dom=document.createElement("div"),this.dom.appendChild(this.announceDOM),this.dom.appendChild(this.scrollDOM),e.parent&&e.parent.appendChild(this.dom);let{dispatch:n}=e;this.dispatchTransactions=e.dispatchTransactions||n&&(s=>s.forEach(r=>n(r,this)))||(s=>this.update(s)),this.dispatch=this.dispatch.bind(this),this._root=e.root||Jd(e.parent)||document,this.viewState=new fa(this,e.state||z.create(e)),e.scrollTo&&e.scrollTo.is(oi)&&(this.viewState.scrollTarget=e.scrollTo.value.clip(this.viewState.state)),this.plugins=this.state.facet(Yt).map(s=>new Ns(s));for(let s of this.plugins)s.update(this);this.observer=new kf(this),this.inputState=new Vu(this),this.inputState.ensureHandlers(this.plugins),this.docView=new ea(this),this.mountStyles(),this.updateAttrs(),this.updateState=0,this.requestMeasure(),!((t=document.fonts)===null||t===void 0)&&t.ready&&document.fonts.ready.then(()=>{this.viewState.mustMeasureContent="refresh",this.requestMeasure()})}dispatch(...e){let t=e.length==1&&e[0]instanceof ae?e:e.length==1&&Array.isArray(e[0])?e[0]:[this.state.update(...e)];this.dispatchTransactions(t,this)}update(e){if(this.updateState!=0)throw new Error("Calls to EditorView.update are not allowed while an update is in progress");let t=!1,n=!1,s,r=this.state;for(let u of e){if(u.startState!=r)throw new RangeError("Trying to update state with a transaction that doesn't start from the previous state.");r=u.state}if(this.destroyed){this.viewState.state=r;return}let o=this.hasFocus,a=0,l=null;e.some(u=>u.annotation(vh))?(this.inputState.notifiedFocused=o,a=1):o!=this.inputState.notifiedFocused&&(this.inputState.notifiedFocused=o,l=kh(r,o),l||(a=1));let h=this.observer.delayedAndroidKey,c=null;if(h?(this.observer.clearDelayedAndroidKey(),c=this.observer.readChange(),(c&&!this.state.doc.eq(r.doc)||!this.state.selection.eq(r.selection))&&(c=null)):this.observer.clear(),r.facet(z.phrases)!=this.state.facet(z.phrases))return this.setState(r);s=Fi.create(this,r,e),s.flags|=a;let d=this.viewState.scrollTarget;try{this.updateState=2;for(let u of e){if(d&&(d=d.map(u.changes)),u.scrollIntoView){let{main:f}=u.state.selection,{x:p,y:m}=this.state.facet(R.cursorScrollMargin);d=new tn(f.empty?f:b.cursor(f.head,f.head>f.anchor?-1:1),"nearest","nearest",m,p)}for(let f of u.effects)f.is(oi)&&(d=f.value.clip(this.state))}this.viewState.update(s,d),this.bidiCache=$i.update(this.bidiCache,s.changes),s.empty||(this.updatePlugins(s),this.inputState.update(s)),t=this.docView.update(s),this.state.facet(On)!=this.styleModules&&this.mountStyles(),n=this.updateAttrs(),this.showAnnouncements(e),this.docView.updateSelection(t,e.some(u=>u.isUserEvent("select.pointer")))}finally{this.updateState=0}if(s.startState.facet(ui)!=s.state.facet(ui)&&(this.viewState.mustMeasureContent=!0),(t||n||d||this.viewState.mustEnforceCursorAssoc||this.viewState.mustMeasureContent)&&this.requestMeasure(),t&&this.docViewUpdate(),!s.empty)for(let u of this.state.facet(Mr))try{u(s)}catch(f){Ce(this.state,f,"update listener")}(l||c)&&Promise.resolve().then(()=>{l&&this.state==l.startState&&this.dispatch(l),c&&!fh(this,c)&&h.force&&en(this.contentDOM,h.key,h.keyCode)})}setState(e){if(this.updateState!=0)throw new Error("Calls to EditorView.setState are not allowed while an update is in progress");if(this.destroyed){this.viewState.state=e;return}this.updateState=2;let t=this.hasFocus;try{for(let n of this.plugins)n.destroy(this);this.viewState=new fa(this,e),this.plugins=e.facet(Yt).map(n=>new Ns(n)),this.pluginMap.clear();for(let n of this.plugins)n.update(this);this.docView.destroy(),this.docView=new ea(this),this.inputState.ensureHandlers(this.plugins),this.mountStyles(),this.updateAttrs(),this.bidiCache=[]}finally{this.updateState=0}t&&this.focus(),this.requestMeasure()}updatePlugins(e){let t=e.startState.facet(Yt),n=e.state.facet(Yt);if(t!=n){let s=[];for(let r of n){let o=t.indexOf(r);if(o<0)s.push(new Ns(r));else{let a=this.plugins[o];a.mustUpdate=e,s.push(a)}}for(let r of this.plugins)r.mustUpdate!=e&&r.destroy(this);this.plugins=s,this.pluginMap.clear()}else for(let s of this.plugins)s.mustUpdate=e;for(let s=0;s<this.plugins.length;s++)this.plugins[s].update(this);t!=n&&this.inputState.ensureHandlers(this.plugins)}docViewUpdate(){for(let e of this.plugins){let t=e.value;if(t&&t.docViewUpdate)try{t.docViewUpdate(this)}catch(n){Ce(this.state,n,"doc view update listener")}}}measure(e=!0){if(this.destroyed)return;if(this.measureScheduled>-1&&this.win.cancelAnimationFrame(this.measureScheduled),this.observer.delayedAndroidKey){this.measureScheduled=-1,this.requestMeasure();return}this.measureScheduled=0,e&&this.observer.forceFlush();let t=null,n=this.viewState.scrollParent,s=this.viewState.getScrollOffset(),{scrollAnchorPos:r,scrollAnchorHeight:o,scaleY:a}=this.viewState;Math.abs(s-this.viewState.scrollOffset)>1&&(o=-1),this.viewState.scrollAnchorHeight=-1;try{for(let l=0;;l++){if(o<0){if(Fl(n||this.win))r=-1,o=this.viewState.heightMap.height/this.viewState.scaleY;else{let p=this.viewState.scrollAnchorAt(s);r=p.from,o=p.top}a=this.viewState.scaleY}this.updateState=1;let h=this.viewState.measure();if(!h&&!this.measureRequests.length&&this.viewState.scrollTarget==null)break;if(l>5){console.warn(this.measureRequests.length?"Measure loop restarted more than 5 times":"Viewport failed to stabilize");break}let c=[];h&4||([this.measureRequests,c]=[c,this.measureRequests]);let d=c.map(p=>{try{return p.read(this)}catch(m){return Ce(this.state,m),ba}}),u=Fi.create(this,this.state,[]),f=!1;u.flags|=h,t?t.flags|=h:t=u,this.updateState=2,u.empty||(this.updatePlugins(u),this.inputState.update(u),this.updateAttrs(),f=this.docView.update(u),f&&this.docViewUpdate());for(let p=0;p<c.length;p++)if(d[p]!=ba)try{let m=c[p];m.write&&m.write(d[p],this)}catch(m){Ce(this.state,m)}if(f&&this.docView.updateSelection(!0),!u.viewportChanged&&this.measureRequests.length==0){if(this.viewState.editorHeight)if(this.viewState.scrollTarget){this.docView.scrollIntoView(this.viewState.scrollTarget),this.viewState.scrollTarget=null,o=-1;continue}else{let m=(r<0?this.viewState.heightMap.height:this.viewState.lineBlockAt(r).top)/this.viewState.scaleY-o/a;if((m>1||m<-1)&&!(O.ios&&this.inputState.lastIOSMomentumScroll>Date.now()-100)&&(n==this.scrollDOM||this.hasFocus||Math.max(this.inputState.lastWheelEvent,this.inputState.lastTouchTime)>Date.now()-100)){s=s+m,n?r<0?n.scrollTop=n.scrollHeight:n.scrollTop+=m:this.win.scrollBy(0,m),o=-1;continue}}break}}}finally{this.updateState=0,this.measureScheduled=-1}if(t&&!t.empty)for(let l of this.state.facet(Mr))l(t)}get themeClasses(){return Lr+" "+(this.state.facet(Er)?Ah:Ch)+" "+this.state.facet(ui)}updateAttrs(){let e=wa(this,rh,{class:"cm-editor"+(this.hasFocus?" cm-focused ":" ")+this.themeClasses}),t={spellcheck:"false",autocorrect:"off",autocapitalize:"off",writingsuggestions:"false",translate:"no",contenteditable:this.state.facet(ut)?"true":"false",class:"cm-content",style:`${O.tabSize}: ${this.state.tabSize}`,role:"textbox","aria-multiline":"true"};this.state.readOnly&&(t["aria-readonly"]="true"),wa(this,so,t);let n=this.observer.ignore(()=>{let s=Go(this.contentDOM,this.contentAttrs,t),r=Go(this.dom,this.editorAttrs,e);return s||r});return this.editorAttrs=e,this.contentAttrs=t,n}showAnnouncements(e){let t=!0;for(let n of e)for(let s of n.effects)if(s.is(R.announce)){t&&(this.announceDOM.textContent="",this.win.clearTimeout(this.clearAnnouncement),this.clearAnnouncement=this.win.setTimeout(()=>{this.announceDOM.textContent=" "},200),t=!1);let r=this.announceDOM.appendChild(document.createElement("div"));r.textContent=s.value}}mountStyles(){this.styleModules=this.state.facet(On);let e=this.state.facet(R.cspNonce);on.mount(this.root,this.styleModules.concat(wf).reverse(),e?{nonce:e}:void 0)}readMeasured(){if(this.updateState==2)throw new Error("Reading the editor layout isn't allowed during an update");this.updateState==0&&this.measureScheduled>-1&&this.measure(!1)}requestMeasure(e){if(this.measureScheduled<0&&(this.measureScheduled=this.win.requestAnimationFrame(()=>this.measure())),e){if(this.measureRequests.indexOf(e)>-1)return;if(e.key!=null){for(let t=0;t<this.measureRequests.length;t++)if(this.measureRequests[t].key===e.key){this.measureRequests[t]=e;return}}this.measureRequests.push(e)}}plugin(e){let t=this.pluginMap.get(e);return(t===void 0||t&&t.plugin!=e)&&this.pluginMap.set(e,t=this.plugins.find(n=>n.plugin==e)||null),t&&t.update(this).value}get documentTop(){return this.contentDOM.getBoundingClientRect().top+this.viewState.paddingTop}get documentPadding(){return{top:this.viewState.paddingTop,bottom:this.viewState.paddingBottom}}get scaleX(){return this.viewState.scaleX}get scaleY(){return this.viewState.scaleY}elementAtHeight(e){return this.readMeasured(),this.viewState.elementAtHeight(e)}lineBlockAtHeight(e){return this.readMeasured(),this.viewState.lineBlockAtHeight(e)}get viewportLineBlocks(){return this.viewState.viewportLines}lineBlockAt(e){return this.viewState.lineBlockAt(e)}get contentHeight(){return this.viewState.contentHeight}moveByChar(e,t,n){return Fs(this,e,ta(this,e,t,n))}moveByGroup(e,t){return Fs(this,e,ta(this,e,t,n=>Eu(this,e.head,n)))}visualLineSide(e,t){return t?b.cursor(e.to,1):b.cursor(e.from,-1)}moveToLineBoundary(e,t,n=!0){return Ru(this,e,t,n)}moveVertically(e,t,n){return Fs(this,e,Lu(this,e,t,n))}domAtPos(e,t=1){return this.docView.domAtPos(e,t)}posAtDOM(e,t=0){return this.docView.posFromDOM(e,t)}posAtCoords(e,t=!0){this.readMeasured();let n=Dr(this,e,t);return n&&n.pos}posAndSideAtCoords(e,t=!0){return this.readMeasured(),Dr(this,e,t)}coordsAtPos(e,t=1){this.readMeasured();let n=this.state.doc.lineAt(e),s=this.bidiSpans(n),r=s[at.find(s,e-n.from,-1,t)];return n.length&&(e==n.from&&t<0||e==n.to&&t>0)&&r.dir!=this.textDirectionAt(n.from)&&(e==n.to?(e=n.from+r.from,t=1):(e=n.from+r.to,t=-1)),this.docView.coordsAt(e,t,r.dir==J.RTL)}coordsForChar(e){return this.readMeasured(),this.docView.coordsForChar(e)}get defaultCharacterWidth(){return this.viewState.heightOracle.charWidth}get defaultLineHeight(){return this.viewState.heightOracle.lineHeight}get textDirection(){return this.viewState.defaultTextDirection}textDirectionAt(e){return!this.state.facet(th)||e<this.viewport.from||e>this.viewport.to?this.textDirection:(this.readMeasured(),this.docView.textDirectionAt(e))}get lineWrapping(){return this.viewState.heightOracle.lineWrapping}bidiSpans(e){if(e.length>Cf)return Kl(e.length);let t=this.textDirectionAt(e.from),n;for(let r of this.bidiCache)if(r.from==e.from&&r.dir==t&&(r.fresh||$l(r.isolates,n=Xo(this,e))))return r.order;n||(n=Xo(this,e));let s=su(e.text,t,n);return this.bidiCache.push(new $i(e.from,e.to,t,n,!0,s)),s}get hasFocus(){var e;return(this.dom.ownerDocument.hasFocus()||O.safari&&((e=this.inputState)===null||e===void 0?void 0:e.lastContextMenu)>Date.now()-3e4)&&this.root.activeElement==this.contentDOM}focus(){this.observer.ignore(()=>{Hl(this.contentDOM),this.docView.updateSelection()})}setRoot(e){this._root!=e&&(this._root=e,this.observer.setWindow((e.nodeType==9?e:e.ownerDocument).defaultView||window),this.mountStyles())}destroy(){this.root.activeElement==this.contentDOM&&this.contentDOM.blur();for(let e of this.plugins)e.destroy(this);this.plugins=[],this.inputState.destroy(),this.docView.destroy(),this.dom.remove(),this.observer.destroy(),this.win.clearTimeout(this.clearAnnouncement),this.measureScheduled>-1&&this.win.cancelAnimationFrame(this.measureScheduled),this.destroyed=!0}static scrollIntoView(e,t={}){var n,s,r,o;return oi.of(new tn(typeof e=="number"?b.cursor(e):e,(n=t.y)!==null&&n!==void 0?n:"nearest",(s=t.x)!==null&&s!==void 0?s:"nearest",(r=t.yMargin)!==null&&r!==void 0?r:5,(o=t.xMargin)!==null&&o!==void 0?o:5))}scrollSnapshot(){let{scrollTop:e,scrollLeft:t}=this.scrollDOM,n=this.viewState.scrollAnchorAt(e);return oi.of(new tn(b.cursor(n.from),"start","start",n.top-e,t,!0))}setTabFocusMode(e){e==null?this.inputState.tabFocusMode=this.inputState.tabFocusMode<0?0:-1:typeof e=="boolean"?this.inputState.tabFocusMode=e?0:-1:this.inputState.tabFocusMode!=0&&(this.inputState.tabFocusMode=Date.now()+e)}static domEventHandlers(e){return ce.define(()=>({}),{eventHandlers:e})}static domEventObservers(e){return ce.define(()=>({}),{eventObservers:e})}static theme(e,t){let n=on.newName(),s=[ui.of(n),On.of(Br(`.${n}`,e))];return t&&t.dark&&s.push(Er.of(!0)),s}static baseTheme(e){return pn.lowest(On.of(Br("."+Lr,e,Th)))}static findFromDOM(e){var t;let n=e.querySelector(".cm-content"),s=n&&te.get(n)||te.get(e);return((t=s==null?void 0:s.root)===null||t===void 0?void 0:t.view)||null}}R.styleModule=On;R.inputHandler=Zl;R.clipboardInputFilter=no;R.clipboardOutputFilter=io;R.scrollHandler=ih;R.focusChangeEffect=eh;R.perLineTextDirection=th;R.exceptionSink=Ql;R.updateListener=Mr;R.editable=ut;R.mouseSelectionStyle=Xl;R.dragMovesSelection=Jl;R.clickAddsSelectionRange=Yl;R.decorations=ds;R.blockWrappers=oh;R.outerDecorations=ro;R.atomicRanges=ei;R.bidiIsolatedRanges=ah;R.cursorScrollMargin=D.define({combine:i=>{let e=5,t=5;for(let n of i)typeof n=="number"?e=t=n:{x:e,y:t}=n;return{x:e,y:t}}});R.scrollMargins=lh;R.darkTheme=Er;R.cspNonce=D.define({combine:i=>i.length?i[0]:""});R.contentAttributes=so;R.editorAttributes=rh;R.lineWrapping=R.contentAttributes.of({class:"cm-lineWrapping"});R.announce=N.define();const Cf=4096,ba={};class $i{constructor(e,t,n,s,r,o){this.from=e,this.to=t,this.dir=n,this.isolates=s,this.fresh=r,this.order=o}static update(e,t){if(t.empty&&!e.some(r=>r.fresh))return e;let n=[],s=e.length?e[e.length-1].dir:J.LTR;for(let r=Math.max(0,e.length-10);r<e.length;r++){let o=e[r];o.dir==s&&!t.touchesRange(o.from,o.to)&&n.push(new $i(t.mapPos(o.from,1),t.mapPos(o.to,-1),o.dir,o.isolates,!1,o.order))}return n}}function wa(i,e,t){for(let n=i.state.facet(e),s=n.length-1;s>=0;s--){let r=n[s],o=typeof r=="function"?r(i):r;o&&Zr(o,t)}return t}const Af=O.mac?"mac":O.windows?"win":O.linux?"linux":"key";function Tf(i,e){const t=i.split(/-(?!$)/);let n=t[t.length-1];n=="Space"&&(n=" ");let s,r,o,a;for(let l=0;l<t.length-1;++l){const h=t[l];if(/^(cmd|meta|m)$/i.test(h))a=!0;else if(/^a(lt)?$/i.test(h))s=!0;else if(/^(c|ctrl|control)$/i.test(h))r=!0;else if(/^s(hift)?$/i.test(h))o=!0;else if(/^mod$/i.test(h))e=="mac"?a=!0:r=!0;else throw new Error("Unrecognized modifier name: "+h)}return s&&(n="Alt-"+n),r&&(n="Ctrl-"+n),a&&(n="Meta-"+n),o&&(n="Shift-"+n),n}function fi(i,e,t){return e.altKey&&(i="Alt-"+i),e.ctrlKey&&(i="Ctrl-"+i),e.metaKey&&(i="Meta-"+i),t!==!1&&e.shiftKey&&(i="Shift-"+i),i}const Mf=pn.default(R.domEventHandlers({keydown(i,e){return Oh(Mh(e.state),i,e,"editor")}})),co=D.define({enables:Mf}),va=new WeakMap;function Mh(i){let e=i.facet(co),t=va.get(e);return t||va.set(e,t=Df(e.reduce((n,s)=>n.concat(s),[]))),t}function Of(i,e,t){return Oh(Mh(i.state),e,i,t)}let xt=null;const If=4e3;function Df(i,e=Af){let t=Object.create(null),n=Object.create(null),s=(o,a)=>{let l=n[o];if(l==null)n[o]=a;else if(l!=a)throw new Error("Key binding "+o+" is used both as a regular binding and as a multi-stroke prefix")},r=(o,a,l,h,c)=>{var d,u;let f=t[o]||(t[o]=Object.create(null)),p=a.split(/ (?!$)/).map(y=>Tf(y,e));for(let y=1;y<p.length;y++){let x=p.slice(0,y).join(" ");s(x,!0),f[x]||(f[x]={preventDefault:!0,stopPropagation:!1,run:[v=>{let I=xt={view:v,prefix:x,scope:o};return setTimeout(()=>{xt==I&&(xt=null)},If),!0}]})}let m=p.join(" ");s(m,!1);let g=f[m]||(f[m]={preventDefault:!1,stopPropagation:!1,run:((u=(d=f._any)===null||d===void 0?void 0:d.run)===null||u===void 0?void 0:u.slice())||[]});l&&g.run.push(l),h&&(g.preventDefault=!0),c&&(g.stopPropagation=!0)};for(let o of i){let a=o.scope?o.scope.split(" "):["editor"];if(o.any)for(let h of a){let c=t[h]||(t[h]=Object.create(null));c._any||(c._any={preventDefault:!1,stopPropagation:!1,run:[]});let{any:d}=o;for(let u in c)c[u].run.push(f=>d(f,_r))}let l=o[e]||o.key;if(l)for(let h of a)r(h,l,o.run,o.preventDefault,o.stopPropagation),o.shift&&r(h,"Shift-"+l,o.shift,o.preventDefault,o.stopPropagation)}return t}let _r=null;function Oh(i,e,t,n){_r=e;let s=zd(e),r=_e(s,0),o=rt(r)==s.length&&s!=" ",a="",l=!1,h=!1,c=!1;xt&&xt.view==t&&xt.scope==n&&(a=xt.prefix+" ",gh.indexOf(e.keyCode)<0&&(h=!0,xt=null));let d=new Set,u=g=>{if(g){for(let y of g.run)if(!d.has(y)&&(d.add(y),y(t)))return g.stopPropagation&&(c=!0),!0;g.preventDefault&&(g.stopPropagation&&(c=!0),h=!0)}return!1},f=i[n],p,m;return f&&(u(f[a+fi(s,e,!o)])?l=!0:o&&(e.altKey||e.metaKey||e.ctrlKey)&&!(O.windows&&e.ctrlKey&&e.altKey)&&!(O.mac&&e.altKey&&!(e.ctrlKey||e.metaKey))&&(p=Ot[e.keyCode])&&p!=s?(u(f[a+fi(p,e,!0)])||e.shiftKey&&(m=Fn[e.keyCode])!=s&&m!=p&&u(f[a+fi(m,e,!1)]))&&(l=!0):o&&e.shiftKey&&u(f[a+fi(s,e,!0)])&&(l=!0),!l&&u(f._any)&&(l=!0)),h&&(l=!0),l&&c&&e.stopPropagation(),_r=null,l}class qt{constructor(e,t,n,s,r){this.className=e,this.left=t,this.top=n,this.width=s,this.height=r}draw(){let e=document.createElement("div");return e.className=this.className,this.adjust(e),e}update(e,t){return t.className!=this.className?!1:(this.adjust(e),!0)}adjust(e){e.style.left=this.left+"px",e.style.top=this.top+"px",this.width!=null&&(e.style.width=this.width+"px"),e.style.height=this.height+"px"}eq(e){return this.left==e.left&&this.top==e.top&&this.width==e.width&&this.height==e.height&&this.className==e.className}static forRange(e,t,n){if(n.empty){let s=e.coordsAtPos(n.head,n.assoc||1);if(!s)return[];let r=Ih(e);return[new qt(t,s.left-r.left,s.top-r.top,null,s.bottom-s.top)]}else return Rf(e,t,n)}}function Ih(i){let e=i.scrollDOM.getBoundingClientRect();return{left:(i.textDirection==J.LTR?e.left:e.right-i.scrollDOM.clientWidth*i.scaleX)-i.scrollDOM.scrollLeft*i.scaleX,top:e.top-i.scrollDOM.scrollTop*i.scaleY}}function ka(i,e,t,n){let s=i.coordsAtPos(e,t*2);if(!s)return n;let r=i.dom.getBoundingClientRect(),o=(s.top+s.bottom)/2,a=i.posAtCoords({x:r.left+1,y:o}),l=i.posAtCoords({x:r.right-1,y:o});return a==null||l==null?n:{from:Math.max(n.from,Math.min(a,l)),to:Math.min(n.to,Math.max(a,l))}}function Rf(i,e,t){if(t.to<=i.viewport.from||t.from>=i.viewport.to)return[];let n=Math.max(t.from,i.viewport.from),s=Math.min(t.to,i.viewport.to),r=i.textDirection==J.LTR,o=i.contentDOM,a=o.getBoundingClientRect(),l=Ih(i),h=o.querySelector(".cm-line"),c=h&&window.getComputedStyle(h),d=a.left+(c?parseInt(c.paddingLeft)+Math.min(0,parseInt(c.textIndent)):0),u=a.right-(c?parseInt(c.paddingRight):0),f=Ir(i,n,1),p=Ir(i,s,-1),m=f.type==pe.Text?f:null,g=p.type==pe.Text?p:null;if(m&&(i.lineWrapping||f.widgetLineBreaks)&&(m=ka(i,n,1,m)),g&&(i.lineWrapping||p.widgetLineBreaks)&&(g=ka(i,s,-1,g)),m&&g&&m.from==g.from&&m.to==g.to)return x(v(t.from,t.to,m));{let S=m?v(t.from,null,m):I(f,!1),C=g?v(null,t.to,g):I(p,!0),A=[];return(m||f).to<(g||p).from-(m&&g?1:0)||f.widgetLineBreaks>1&&S.bottom+i.defaultLineHeight/2<C.top?A.push(y(d,S.bottom,u,C.top)):S.bottom<C.top&&i.elementAtHeight((S.bottom+C.top)/2).type==pe.Text&&(S.bottom=C.top=(S.bottom+C.top)/2),x(S).concat(A).concat(x(C))}function y(S,C,A,L){return new qt(e,S-l.left,C-l.top,Math.max(0,A-S),L-C)}function x({top:S,bottom:C,horizontal:A}){let L=[];for(let q=0;q<A.length;q+=2)L.push(y(A[q],S,A[q+1],C));return L}function v(S,C,A){let L=1e9,q=-1e9,F=[];function W(V,K,ee,ke,Ee){let re=i.coordsAtPos(V,V==A.to?-2:2),Me=i.coordsAtPos(ee,ee==A.from?2:-2);!re||!Me||(L=Math.min(re.top,Me.top,L),q=Math.max(re.bottom,Me.bottom,q),Ee==J.LTR?F.push(r&&K?d:re.left,r&&ke?u:Me.right):F.push(!r&&ke?d:Me.left,!r&&K?u:re.right))}let E=S??A.from,$=C??A.to;for(let V of i.visibleRanges)if(V.to>E&&V.from<$)for(let K=Math.max(V.from,E),ee=Math.min(V.to,$);;){let ke=i.state.doc.lineAt(K);for(let Ee of i.bidiSpans(ke)){let re=Ee.from+ke.from,Me=Ee.to+ke.from;if(re>=ee)break;Me>K&&W(Math.max(re,K),S==null&&re<=E,Math.min(Me,ee),C==null&&Me>=$,Ee.dir)}if(K=ke.to+1,K>=ee)break}return F.length==0&&W(E,S==null,$,C==null,i.textDirection),{top:L,bottom:q,horizontal:F}}function I(S,C){let A=a.top+(C?S.top:S.bottom);return{top:A,bottom:A,horizontal:[]}}}function Ef(i,e){return i.constructor==e.constructor&&i.eq(e)}class Lf{constructor(e,t){this.view=e,this.layer=t,this.drawn=[],this.scaleX=1,this.scaleY=1,this.measureReq={read:this.measure.bind(this),write:this.draw.bind(this)},this.dom=e.scrollDOM.appendChild(document.createElement("div")),this.dom.classList.add("cm-layer"),t.above&&this.dom.classList.add("cm-layer-above"),t.class&&this.dom.classList.add(t.class),this.scale(),this.dom.setAttribute("aria-hidden","true"),this.setOrder(e.state),e.requestMeasure(this.measureReq),t.mount&&t.mount(this.dom,e)}update(e){e.startState.facet(Ri)!=e.state.facet(Ri)&&this.setOrder(e.state),(this.layer.update(e,this.dom)||e.geometryChanged)&&(this.scale(),e.view.requestMeasure(this.measureReq))}docViewUpdate(e){this.layer.updateOnDocViewUpdate!==!1&&e.requestMeasure(this.measureReq)}setOrder(e){let t=0,n=e.facet(Ri);for(;t<n.length&&n[t]!=this.layer;)t++;this.dom.style.zIndex=String((this.layer.above?150:-1)-t)}measure(){return this.layer.markers(this.view)}scale(){let{scaleX:e,scaleY:t}=this.view;(e!=this.scaleX||t!=this.scaleY)&&(this.scaleX=e,this.scaleY=t,this.dom.style.transform=`scale(${1/e}, ${1/t})`)}draw(e){if(e.length!=this.drawn.length||e.some((t,n)=>!Ef(t,this.drawn[n]))){let t=this.dom.firstChild,n=0;for(let s of e)s.update&&t&&s.constructor&&this.drawn[n].constructor&&s.update(t,this.drawn[n])?(t=t.nextSibling,n++):this.dom.insertBefore(s.draw(),t);for(;t;){let s=t.nextSibling;t.remove(),t=s}this.drawn=e,O.webkit&&(this.dom.style.display=this.dom.firstChild?"":"none")}}destroy(){this.layer.destroy&&this.layer.destroy(this.dom,this.view),this.dom.remove()}}const Ri=D.define();function Dh(i){return[ce.define(e=>new Lf(e,i)),Ri.of(i)]}const cn=D.define({combine(i){return bt(i,{cursorBlinkRate:1200,drawRangeCursor:!0,iosSelectionHandles:!0},{cursorBlinkRate:(e,t)=>Math.min(e,t),drawRangeCursor:(e,t)=>e||t})}});function Bf(i={}){return[cn.of(i),_f,Pf,Wf,nh.of(!0)]}function Rh(i){return i.startState.facet(cn)!=i.state.facet(cn)}const _f=Dh({above:!0,markers(i){let{state:e}=i,t=e.facet(cn),n=[];for(let s of e.selection.ranges){let r=s==e.selection.main;if(s.empty||t.drawRangeCursor&&!(r&&O.ios&&t.iosSelectionHandles)){let o=r?"cm-cursor cm-cursor-primary":"cm-cursor cm-cursor-secondary",a=s.empty?s:b.cursor(s.head,s.assoc);for(let l of qt.forRange(i,o,a))n.push(l)}}return n},update(i,e){i.transactions.some(n=>n.selection)&&(e.style.animationName=e.style.animationName=="cm-blink"?"cm-blink2":"cm-blink");let t=Rh(i);return t&&xa(i.state,e),i.docChanged||i.selectionSet||t},mount(i,e){xa(e.state,i)},class:"cm-cursorLayer"});function xa(i,e){e.style.animationDuration=i.facet(cn).cursorBlinkRate+"ms"}const Pf=Dh({above:!1,markers(i){let e=[],{main:t,ranges:n}=i.state.selection;for(let s of n)if(!s.empty)for(let r of qt.forRange(i,"cm-selectionBackground",s))e.push(r);if(O.ios&&!t.empty&&i.state.facet(cn).iosSelectionHandles){for(let s of qt.forRange(i,"cm-selectionHandle cm-selectionHandle-start",b.cursor(t.from,1)))e.push(s);for(let s of qt.forRange(i,"cm-selectionHandle cm-selectionHandle-end",b.cursor(t.to,1)))e.push(s)}return e},update(i,e){return i.docChanged||i.selectionSet||i.viewportChanged||Rh(i)},class:"cm-selectionLayer"}),Nf=O.gecko&&O.gecko_version==153?"#ffffff01":"transparent",Wf=pn.highest(R.theme({".cm-line":{"& ::selection, &::selection":{backgroundColor:`${Nf} !important`},caretColor:"transparent !important"},".cm-content":{caretColor:"transparent !important","& :focus":{caretColor:"initial !important","&::selection, & ::selection":{backgroundColor:"Highlight !important"}}}}));function qf(){return Ff}const Hf=_.line({class:"cm-activeLine"}),Ff=ce.fromClass(class{constructor(i){this.decorations=this.getDeco(i)}update(i){(i.docChanged||i.selectionSet)&&(this.decorations=this.getDeco(i.view))}getDeco(i){let e=-1,t=[];for(let n of i.state.selection.ranges){let s=i.lineBlockAt(n.head);s.from>e&&(t.push(Hf.range(s.from)),e=s.from)}return _.set(t)}},{decorations:i=>i.decorations}),pi="-10000px";class Eh{constructor(e,t,n,s){this.facet=t,this.createTooltipView=n,this.removeTooltipView=s,this.input=e.state.facet(t),this.tooltips=this.input.filter(o=>o);let r=null;this.tooltipViews=this.tooltips.map(o=>r=n(o,r))}update(e,t){var n;let s=e.state.facet(this.facet),r=s.filter(l=>l);if(s===this.input){for(let l of this.tooltipViews)l.update&&l.update(e);return!1}let o=[],a=t?[]:null;for(let l=0;l<r.length;l++){let h=r[l],c=-1;if(h){for(let d=0;d<this.tooltips.length;d++){let u=this.tooltips[d];u&&u.create==h.create&&(c=d)}if(c<0)o[l]=this.createTooltipView(h,l?o[l-1]:null),a&&(a[l]=!!h.above);else{let d=o[l]=this.tooltipViews[c];a&&(a[l]=t[c]),d.update&&d.update(e)}}}for(let l of this.tooltipViews)o.indexOf(l)<0&&(this.removeTooltipView(l),(n=l.destroy)===null||n===void 0||n.call(l));return t&&(a.forEach((l,h)=>t[h]=l),t.length=a.length),this.input=s,this.tooltips=r,this.tooltipViews=o,!0}}function Vf(i){let e=i.dom.ownerDocument.documentElement;return{top:0,left:0,bottom:e.clientHeight,right:e.clientWidth}}const js=D.define({combine:i=>{var e,t,n;return{position:O.ios?"absolute":((e=i.find(s=>s.position))===null||e===void 0?void 0:e.position)||"fixed",parent:((t=i.find(s=>s.parent))===null||t===void 0?void 0:t.parent)||null,tooltipSpace:((n=i.find(s=>s.tooltipSpace))===null||n===void 0?void 0:n.tooltipSpace)||Vf}}}),Sa=new WeakMap,uo=ce.fromClass(class{constructor(i){this.view=i,this.above=[],this.inView=!0,this.madeAbsolute=!1,this.lastTransaction=0,this.measureTimeout=-1;let e=i.state.facet(js);this.position=e.position,this.parent=e.parent,this.classes=i.themeClasses,this.createContainer(),this.measureReq={read:this.readMeasure.bind(this),write:this.writeMeasure.bind(this),key:this},this.resizeObserver=typeof ResizeObserver=="function"?new ResizeObserver(()=>this.measureSoon()):null,this.manager=new Eh(i,ms,(t,n)=>this.createTooltip(t,n),t=>{this.resizeObserver&&this.resizeObserver.unobserve(t.dom),t.dom.remove()}),this.above=this.manager.tooltips.map(t=>!!t.above),this.intersectionObserver=typeof IntersectionObserver=="function"?new IntersectionObserver(t=>{Date.now()>this.lastTransaction-50&&t.length>0&&t[t.length-1].intersectionRatio<1&&this.measureSoon()},{threshold:[1]}):null,this.observeIntersection(),i.win.addEventListener("resize",this.measureSoon=this.measureSoon.bind(this)),this.maybeMeasure()}createContainer(){this.parent?(this.container=document.createElement("div"),this.container.style.position="relative",this.container.className=this.view.themeClasses,this.parent.appendChild(this.container)):this.container=this.view.dom}observeIntersection(){if(this.intersectionObserver){this.intersectionObserver.disconnect();for(let i of this.manager.tooltipViews)this.intersectionObserver.observe(i.dom)}}measureSoon(){this.measureTimeout<0&&(this.measureTimeout=setTimeout(()=>{this.measureTimeout=-1,this.maybeMeasure()},50))}update(i){i.transactions.length&&(this.lastTransaction=Date.now());let e=this.manager.update(i,this.above);e&&this.observeIntersection();let t=e||i.geometryChanged,n=i.state.facet(js);if(n.position!=this.position&&!this.madeAbsolute){this.position=n.position;for(let s of this.manager.tooltipViews)s.dom.style.position=this.position;t=!0}if(n.parent!=this.parent){this.parent&&this.container.remove(),this.parent=n.parent,this.createContainer();for(let s of this.manager.tooltipViews)this.container.appendChild(s.dom);t=!0}else this.parent&&this.view.themeClasses!=this.classes&&(this.classes=this.container.className=this.view.themeClasses);t&&this.maybeMeasure()}createTooltip(i,e){let t=i.create(this.view),n=e?e.dom:null;if(t.dom.classList.add("cm-tooltip"),i.arrow&&!t.dom.querySelector(".cm-tooltip > .cm-tooltip-arrow")){let s=document.createElement("div");s.className="cm-tooltip-arrow",t.dom.appendChild(s)}return t.dom.style.position=this.position,t.dom.style.top=pi,t.dom.style.left="0px",this.container.insertBefore(t.dom,n),t.mount&&t.mount(this.view),this.resizeObserver&&this.resizeObserver.observe(t.dom),t}destroy(){var i,e,t;this.view.win.removeEventListener("resize",this.measureSoon);for(let n of this.manager.tooltipViews)n.dom.remove(),(i=n.destroy)===null||i===void 0||i.call(n);this.parent&&this.container.remove(),(e=this.resizeObserver)===null||e===void 0||e.disconnect(),(t=this.intersectionObserver)===null||t===void 0||t.disconnect(),clearTimeout(this.measureTimeout)}readMeasure(){let i=1,e=1,t=!1;if(this.position=="fixed"&&this.manager.tooltipViews.length){let{dom:r}=this.manager.tooltipViews[0];if(O.safari){let o=r.getBoundingClientRect();t=Math.abs(o.top+1e4)>1||Math.abs(o.left)>1}else t=!!r.offsetParent&&r.offsetParent!=this.container.ownerDocument.body}if(t||this.position=="absolute")if(this.parent){let r=this.parent.getBoundingClientRect();r.width&&r.height&&(i=r.width/this.parent.offsetWidth,e=r.height/this.parent.offsetHeight)}else({scaleX:i,scaleY:e}=this.view.viewState);let n=this.view.scrollDOM.getBoundingClientRect(),s=oo(this.view);return{visible:{left:n.left+s.left,top:n.top+s.top,right:n.right-s.right,bottom:n.bottom-s.bottom},parent:this.parent?this.container.getBoundingClientRect():this.view.dom.getBoundingClientRect(),pos:this.manager.tooltips.map((r,o)=>{let a=this.manager.tooltipViews[o];return a.getCoords?a.getCoords(r.pos):this.view.coordsAtPos(r.pos)}),size:this.manager.tooltipViews.map(({dom:r})=>r.getBoundingClientRect()),space:this.view.state.facet(js).tooltipSpace(this.view),scaleX:i,scaleY:e,makeAbsolute:t}}writeMeasure(i){var e;if(i.makeAbsolute){this.madeAbsolute=!0,this.position="absolute";for(let a of this.manager.tooltipViews)a.dom.style.position="absolute"}let{visible:t,space:n,scaleX:s,scaleY:r}=i,o=[];for(let a=0;a<this.manager.tooltips.length;a++){let l=this.manager.tooltips[a],h=this.manager.tooltipViews[a],{dom:c}=h,d=i.pos[a],u=i.size[a];if(!d||l.clip!==!1&&(d.bottom<=Math.max(t.top,n.top)||d.top>=Math.min(t.bottom,n.bottom)||d.right<Math.max(t.left,n.left)-.1||d.left>Math.min(t.right,n.right)+.1)){c.style.top=pi;continue}let f=l.arrow?h.dom.querySelector(".cm-tooltip-arrow"):null,p=f?7:0,m=u.right-u.left,g=(e=Sa.get(h))!==null&&e!==void 0?e:u.bottom-u.top,y=h.offset||jf,x=this.view.textDirection==J.LTR,v=u.width>n.right-n.left?x?n.left:n.right-u.width:x?Math.max(n.left,Math.min(d.left-(f?14:0)+y.x,n.right-m)):Math.min(Math.max(n.left,d.left-m+(f?14:0)-y.x),n.right-m),I=this.above[a];!l.strictSide&&(I?d.top-g-p-y.y<n.top:d.bottom+g+p+y.y>n.bottom)&&I==n.bottom-d.bottom>d.top-n.top&&(I=this.above[a]=!I);let S=(I?d.top-n.top:n.bottom-d.bottom)-p;if(S<g&&h.resize!==!1){if(S<this.view.defaultLineHeight){c.style.top=pi;continue}Sa.set(h,g),c.style.height=(g=S)/r+"px"}else c.style.height&&(c.style.height="");let C=I?d.top-g-p-y.y:d.bottom+p+y.y,A=v+m;if(h.overlap!==!0)for(let L of o)L.left<A&&L.right>v&&L.top<C+g&&L.bottom>C&&(C=I?L.top-g-2-p:L.bottom+p+2);if(this.position=="absolute"?(c.style.top=(C-i.parent.top)/r+"px",Ca(c,(v-i.parent.left)/s)):(c.style.top=C/r+"px",Ca(c,v/s)),f){let L=d.left+(x?y.x:-y.x)-(v+14-7);f.style.left=L/s+"px"}h.overlap!==!0&&o.push({left:v,top:C,right:A,bottom:C+g}),c.classList.toggle("cm-tooltip-above",I),c.classList.toggle("cm-tooltip-below",!I),h.positioned&&h.positioned(i.space)}}maybeMeasure(){if(this.manager.tooltips.length&&(this.view.inView&&this.view.requestMeasure(this.measureReq),this.inView!=this.view.inView&&(this.inView=this.view.inView,!this.inView)))for(let i of this.manager.tooltipViews)i.dom.style.top=pi}},{eventObservers:{scroll(){this.maybeMeasure()}}});function Ca(i,e){let t=parseInt(i.style.left,10);(isNaN(t)||Math.abs(e-t)>1)&&(i.style.left=e+"px")}const zf=R.baseTheme({".cm-tooltip":{zIndex:500,boxSizing:"border-box"},"&light .cm-tooltip":{border:"1px solid #bbb",backgroundColor:"#f5f5f5"},"&light .cm-tooltip-section:not(:first-child)":{borderTop:"1px solid #bbb"},"&dark .cm-tooltip":{backgroundColor:"#333338",color:"white"},".cm-tooltip-arrow":{height:"7px",width:`${7*2}px`,position:"absolute",zIndex:-1,overflow:"hidden","&:before, &:after":{content:"''",position:"absolute",width:0,height:0,borderLeft:"7px solid transparent",borderRight:"7px solid transparent"},".cm-tooltip-above &":{bottom:"-7px","&:before":{borderTop:"7px solid #bbb"},"&:after":{borderTop:"7px solid #f5f5f5",bottom:"1px"}},".cm-tooltip-below &":{top:"-7px","&:before":{borderBottom:"7px solid #bbb"},"&:after":{borderBottom:"7px solid #f5f5f5",top:"1px"}}},"&dark .cm-tooltip .cm-tooltip-arrow":{"&:before":{borderTopColor:"#333338",borderBottomColor:"#333338"},"&:after":{borderTopColor:"transparent",borderBottomColor:"transparent"}}}),jf={x:0,y:0},ms=D.define({enables:[uo,zf]}),Ki=D.define({combine:i=>i.reduce((e,t)=>e.concat(t),[])});class gs{static create(e){return new gs(e)}constructor(e){this.view=e,this.mounted=!1,this.dom=document.createElement("div"),this.dom.classList.add("cm-tooltip-hover"),this.manager=new Eh(e,Ki,(t,n)=>this.createHostedView(t,n),t=>t.dom.remove())}createHostedView(e,t){let n=e.create(this.view);return n.dom.classList.add("cm-tooltip-section"),this.dom.insertBefore(n.dom,t?t.dom.nextSibling:this.dom.firstChild),this.mounted&&n.mount&&n.mount(this.view),n}mount(e){for(let t of this.manager.tooltipViews)t.mount&&t.mount(e);this.mounted=!0}positioned(e){for(let t of this.manager.tooltipViews)t.positioned&&t.positioned(e)}update(e){this.manager.update(e)}destroy(){var e;for(let t of this.manager.tooltipViews)(e=t.destroy)===null||e===void 0||e.call(t)}passProp(e){let t;for(let n of this.manager.tooltipViews){let s=n[e];if(s!==void 0){if(t===void 0)t=s;else if(t!==s)return}}return t}get offset(){return this.passProp("offset")}get getCoords(){return this.passProp("getCoords")}get overlap(){return this.passProp("overlap")}get resize(){return this.passProp("resize")}}const Uf=ms.compute([Ki],i=>{let e=i.facet(Ki);return e.length===0?null:{pos:Math.min(...e.map(t=>t.pos)),end:Math.max(...e.map(t=>{var n;return(n=t.end)!==null&&n!==void 0?n:t.pos})),create:gs.create,above:e[0].above,arrow:e.some(t=>t.arrow)}}),Lh=D.define();class $f{constructor(e,t,n,s,r,o){this.view=e,this.source=t,this.field=n,this.locked=s,this.setHover=r,this.hoverTime=o,this.hoverTimeout=-1,this.restartTimeout=-1,this.pending=null,this.lastMove={x:0,y:0,target:e.dom,time:0},this.checkHover=this.checkHover.bind(this),e.dom.addEventListener("mouseleave",this.mouseleave=this.mouseleave.bind(this)),e.dom.addEventListener("mousemove",this.mousemove=this.mousemove.bind(this))}update(e){this.pending&&(this.pending=null,clearTimeout(this.restartTimeout),this.restartTimeout=setTimeout(()=>this.startHover(),20))}get active(){return this.view.state.field(this.field)}checkHover(){if(this.hoverTimeout=-1,this.active.length)return;let e=Date.now()-this.lastMove.time;e<this.hoverTime?this.hoverTimeout=setTimeout(this.checkHover,this.hoverTime-e):this.startHover()}startHover(){clearTimeout(this.restartTimeout);let{view:e,lastMove:t}=this,n=e.docView.tile.nearest(t.target);if(!n)return;let s,r=1;if(n.isWidget())s=n.posAtStart;else{if(s=e.posAtCoords(t),s==null)return;let o=e.coordsAtPos(s);if(!o||t.y<o.top||t.y>o.bottom||t.x<o.left-e.defaultCharacterWidth||t.x>o.right+e.defaultCharacterWidth)return;let a=e.bidiSpans(e.state.doc.lineAt(s)).find(h=>h.from<=s&&h.to>=s),l=a&&a.dir==J.RTL?-1:1;r=t.x<o.left?-l:l}this.activateHover(e,s,r)}activateHover(e,t,n,s){let r=this.source(e,t,n),o=a=>{if(a&&!(Array.isArray(a)&&!a.length)){let l=Array.isArray(a)?a:[a];s&&this.locked.set(l,s),e.dispatch({effects:this.setHover.of(l)})}};if(r&&"then"in r){let a=this.pending={pos:t};r.then(l=>{this.pending==a&&(this.pending=null,o(l))},l=>Ce(e.state,l,"hover tooltip"))}else o(r)}get tooltip(){let e=this.view.plugin(uo),t=e?e.manager.tooltips.findIndex(n=>n.create==gs.create):-1;return t>-1?e.manager.tooltipViews[t]:null}mousemove(e){var t,n;this.lastMove={x:e.clientX,y:e.clientY,target:e.target,time:Date.now()},this.hoverTimeout<0&&(this.hoverTimeout=setTimeout(this.checkHover,this.hoverTime));let{active:s,tooltip:r}=this;if(s.length&&!this.locked.has(s)&&r&&!Kf(r.dom,e)||this.pending){let{pos:o}=s[0]||this.pending,a=(n=(t=s[0])===null||t===void 0?void 0:t.end)!==null&&n!==void 0?n:o;(o==a?this.view.posAtCoords(this.lastMove)!=o:!Gf(this.view,o,a,e.clientX,e.clientY))&&(this.view.dispatch({effects:this.setHover.of([])}),this.pending=null)}}mouseleave(e){clearTimeout(this.hoverTimeout),this.hoverTimeout=-1;let{active:t}=this;if(t.length&&!this.locked.has(t)){let{tooltip:n}=this;n&&n.dom.contains(e.relatedTarget)?this.watchTooltipLeave(n.dom):this.view.dispatch({effects:this.setHover.of([])})}}watchTooltipLeave(e){let t=n=>{e.removeEventListener("mouseleave",t);let{active:s}=this;s.length&&!this.locked.has(s)&&!this.view.dom.contains(n.relatedTarget)&&this.view.dispatch({effects:this.setHover.of([])})};e.addEventListener("mouseleave",t)}destroy(){clearTimeout(this.hoverTimeout),clearTimeout(this.restartTimeout),this.view.dom.removeEventListener("mouseleave",this.mouseleave),this.view.dom.removeEventListener("mousemove",this.mousemove)}}const mi=4;function Kf(i,e){let{left:t,right:n,top:s,bottom:r}=i.getBoundingClientRect(),o;if(o=i.querySelector(".cm-tooltip-arrow")){let a=o.getBoundingClientRect();s=Math.min(a.top,s),r=Math.max(a.bottom,r)}return e.clientX>=t-mi&&e.clientX<=n+mi&&e.clientY>=s-mi&&e.clientY<=r+mi}function Gf(i,e,t,n,s,r){let o=i.scrollDOM.getBoundingClientRect(),a=i.documentTop+i.documentPadding.top+i.contentHeight;if(o.left>n||o.right<n||o.top>s||Math.min(o.bottom,a)<s)return!1;let l=i.posAtCoords({x:n,y:s},!1);return l>=e&&l<=t}function Bh(i,e={}){let t=N.define(),n=new WeakMap,s=me.define({create(){return[]},update(o,a){let l=n.get(o);if(o.length&&(e.hideOnChange&&(a.docChanged||a.selection)?o=[]:l&&l(a)?o=[]:e.hideOn&&(o=o.filter(h=>!e.hideOn(a,h)))),a.docChanged&&o.length){let h=[];for(let c of o){let d=a.changes.mapPos(c.pos,-1,Se.TrackDel);if(d!=null){let u=Object.assign(Object.create(null),c);u.pos=d,u.end!=null&&(u.end=a.changes.mapPos(u.end)),h.push(u)}}o=h}for(let h of a.effects)h.is(t)&&(o=h.value,l=void 0),(h.is(Jf)&&!h.value||h.value==s)&&(o=[]);return o.length&&l&&n.set(o,l),o},provide:o=>Ki.from(o)});const r=ce.define(o=>new $f(o,i,s,n,t,e.hoverTime||300));return{active:s,extension:[s,r,Lh.of(r),Uf]}}function Yf(i,e,t,n={}){var s;let r=i.state.facet(Lh).map(o=>i.plugin(o)).filter(o=>!!o);if(n.tooltip&&n.tooltip.active){let o=r.find(a=>a.field==n.tooltip.active);o&&(r=[o])}for(let o of r)o.activateHover(i,e,t,(s=n.until)!==null&&s!==void 0?s:()=>!1)}function _h(i,e){let t=i.plugin(uo);if(!t)return null;let n=t.manager.tooltips.indexOf(e);return n<0?null:t.manager.tooltipViews[n]}const Jf=N.define(),Aa=D.define({combine(i){let e,t;for(let n of i)e=e||n.topContainer,t=t||n.bottomContainer;return{topContainer:e,bottomContainer:t}}});function ys(i,e){let t=i.plugin(Ph),n=t?t.specs.indexOf(e):-1;return n>-1?t.panels[n]:null}const Ph=ce.fromClass(class{constructor(i){this.input=i.state.facet(Un),this.specs=this.input.filter(t=>t),this.panels=this.specs.map(t=>t(i));let e=i.state.facet(Aa);this.top=new gi(i,!0,e.topContainer),this.bottom=new gi(i,!1,e.bottomContainer),this.top.sync(this.panels.filter(t=>t.top)),this.bottom.sync(this.panels.filter(t=>!t.top));for(let t of this.panels)t.dom.classList.add("cm-panel"),t.mount&&t.mount()}update(i){let e=i.state.facet(Aa);this.top.container!=e.topContainer&&(this.top.sync([]),this.top=new gi(i.view,!0,e.topContainer)),this.bottom.container!=e.bottomContainer&&(this.bottom.sync([]),this.bottom=new gi(i.view,!1,e.bottomContainer)),this.top.syncClasses(),this.bottom.syncClasses();let t=i.state.facet(Un);if(t!=this.input){let n=t.filter(l=>l),s=[],r=[],o=[],a=[];for(let l of n){let h=this.specs.indexOf(l),c;h<0?(c=l(i.view),a.push(c)):(c=this.panels[h],c.update&&c.update(i)),s.push(c),(c.top?r:o).push(c)}this.specs=n,this.panels=s,this.top.sync(r),this.bottom.sync(o);for(let l of a)l.dom.classList.add("cm-panel"),l.mount&&l.mount()}else for(let n of this.panels)n.update&&n.update(i)}destroy(){this.top.sync([]),this.bottom.sync([])}},{provide:i=>R.scrollMargins.of(e=>{let t=e.plugin(i);return t&&{top:t.top.scrollMargin(),bottom:t.bottom.scrollMargin()}})});class gi{constructor(e,t,n){this.view=e,this.top=t,this.container=n,this.dom=void 0,this.classes="",this.panels=[],this.syncClasses()}sync(e){for(let t of this.panels)t.destroy&&e.indexOf(t)<0&&t.destroy();this.panels=e,this.syncDOM()}syncDOM(){if(this.panels.length==0){this.dom&&(this.dom.remove(),this.dom=void 0);return}if(!this.dom){this.dom=document.createElement("div"),this.dom.className=this.top?"cm-panels cm-panels-top":"cm-panels cm-panels-bottom";let t=this.container||this.view.dom;t.insertBefore(this.dom,this.top?t.firstChild:null)}let e=this.dom.firstChild;for(let t of this.panels)if(t.dom.parentNode==this.dom){for(;e!=t.dom;)e=Ta(e);e=e.nextSibling}else this.dom.insertBefore(t.dom,e);for(;e;)e=Ta(e)}scrollMargin(){return!this.dom||this.container?0:Math.max(0,this.top?this.dom.getBoundingClientRect().bottom-Math.max(0,this.view.scrollDOM.getBoundingClientRect().top):Math.min(innerHeight,this.view.scrollDOM.getBoundingClientRect().bottom)-this.dom.getBoundingClientRect().top)}syncClasses(){if(!(!this.container||this.classes==this.view.themeClasses)){for(let e of this.classes.split(" "))e&&this.container.classList.remove(e);for(let e of(this.classes=this.view.themeClasses).split(" "))e&&this.container.classList.add(e)}}}function Ta(i){let e=i.nextSibling;return i.remove(),e}const Un=D.define({enables:Ph});function Xf(i,e){let t,n=new Promise(o=>t=o),s=o=>Zf(o,e,t);i.state.field(Ei,!1)?i.dispatch({effects:Nh.of(s)}):i.dispatch({effects:N.appendConfig.of(Ei.init(()=>[s]))});let r=Wh.of(s);return{close:r,result:n.then(o=>((i.win.queueMicrotask||(l=>i.win.setTimeout(l,10)))(()=>{i.state.field(Ei).indexOf(s)>-1&&i.dispatch({effects:r})}),o))}}function Qf(i,e){let t=i.state.field(Ei,!1)||[];for(let n of t){let s=ys(i,n);if(s&&s.dom.classList.contains(e))return s}return null}const Ei=me.define({create(){return[]},update(i,e){for(let t of e.effects)t.is(Nh)?i=[t.value].concat(i):t.is(Wh)&&(i=i.filter(n=>n!=t.value));return i},provide:i=>Un.computeN([i],e=>e.field(i))}),Nh=N.define(),Wh=N.define();function Zf(i,e,t){let n=e.content?e.content(i,()=>o(null)):null;if(!n){if(n=G("form"),e.input){let a=G("input",e.input);/^(text|password|number|email|tel|url)$/.test(a.type)&&a.classList.add("cm-textfield"),a.name||(a.name="input"),n.appendChild(G("label",(e.label||"")+": ",a))}else n.appendChild(document.createTextNode(e.label||""));n.appendChild(document.createTextNode(" ")),n.appendChild(G("button",{class:"cm-button",type:"submit"},e.submitLabel||"OK"))}let s=n.nodeName=="FORM"?[n]:n.querySelectorAll("form");for(let a=0;a<s.length;a++){let l=s[a];l.addEventListener("keydown",h=>{h.keyCode==27?(h.preventDefault(),o(null)):h.keyCode==13&&(h.preventDefault(),o(l))}),l.addEventListener("submit",h=>{h.preventDefault(),o(l)})}let r=G("div",n,G("button",{onclick:()=>o(null),"aria-label":i.state.phrase("close"),class:"cm-dialog-close",type:"button"},["×"]));e.class&&(r.className=e.class),r.classList.add("cm-dialog");function o(a){r.contains(r.ownerDocument.activeElement)&&i.focus(),t(a)}return{dom:r,top:e.top,mount:()=>{if(e.focus){let a;typeof e.focus=="string"?a=n.querySelector(e.focus):a=n.querySelector("input")||n.querySelector("button"),a&&"select"in a?a.select():a&&"focus"in a&&a.focus()}}}}class gt extends Mt{compare(e){return this==e||this.constructor==e.constructor&&this.eq(e)}eq(e){return!1}destroy(e){}}gt.prototype.elementClass="";gt.prototype.toDOM=void 0;gt.prototype.mapMode=Se.TrackBefore;gt.prototype.startSide=gt.prototype.endSide=-1;gt.prototype.point=!0;const Li=D.define(),ep=D.define(),tp={class:"",renderEmptyElements:!1,elementStyle:"",markers:()=>H.empty,lineMarker:()=>null,widgetMarker:()=>null,lineMarkerChange:null,initialSpacer:null,updateSpacer:null,domEventHandlers:{},side:"before"},_n=D.define();function np(i){return[qh(),_n.of({...tp,...i})]}const Ma=D.define({combine:i=>i.some(e=>e)});function qh(i){return[ip]}const ip=ce.fromClass(class{constructor(i){this.view=i,this.domAfter=null,this.prevViewport=i.viewport,this.dom=document.createElement("div"),this.dom.className="cm-gutters cm-gutters-before",this.dom.setAttribute("aria-hidden","true"),this.dom.style.minHeight=this.view.contentHeight/this.view.scaleY+"px",this.gutters=i.state.facet(_n).map(e=>new Ia(i,e)),this.fixed=!i.state.facet(Ma);for(let e of this.gutters)e.config.side=="after"?this.getDOMAfter().appendChild(e.dom):this.dom.appendChild(e.dom);this.fixed&&(this.dom.style.position="sticky"),this.syncGutters(!1),i.scrollDOM.insertBefore(this.dom,i.contentDOM)}getDOMAfter(){return this.domAfter||(this.domAfter=document.createElement("div"),this.domAfter.className="cm-gutters cm-gutters-after",this.domAfter.setAttribute("aria-hidden","true"),this.domAfter.style.minHeight=this.view.contentHeight/this.view.scaleY+"px",this.domAfter.style.position=this.fixed?"sticky":"",this.view.scrollDOM.appendChild(this.domAfter)),this.domAfter}update(i){if(this.updateGutters(i)){let e=this.prevViewport,t=i.view.viewport,n=Math.min(e.to,t.to)-Math.max(e.from,t.from);this.syncGutters(n<(t.to-t.from)*.8)}if(i.geometryChanged){let e=this.view.contentHeight/this.view.scaleY+"px";this.dom.style.minHeight=e,this.domAfter&&(this.domAfter.style.minHeight=e)}this.view.state.facet(Ma)!=!this.fixed&&(this.fixed=!this.fixed,this.dom.style.position=this.fixed?"sticky":"",this.domAfter&&(this.domAfter.style.position=this.fixed?"sticky":"")),this.prevViewport=i.view.viewport}syncGutters(i){let e=this.dom.nextSibling;i&&(this.dom.remove(),this.domAfter&&this.domAfter.remove());let t=H.iter(this.view.state.facet(Li),this.view.viewport.from),n=[],s=this.gutters.map(r=>new sp(r,this.view.viewport,-this.view.documentPadding.top));for(let r of this.view.viewportLineBlocks)if(n.length&&(n=[]),Array.isArray(r.type)){let o=!0;for(let a of r.type)if(a.type==pe.Text&&o){Pr(t,n,a.from);for(let l of s)l.line(this.view,a,n);o=!1}else if(a.widget)for(let l of s)l.widget(this.view,a)}else if(r.type==pe.Text){Pr(t,n,r.from);for(let o of s)o.line(this.view,r,n)}else if(r.widget)for(let o of s)o.widget(this.view,r);for(let r of s)r.finish();i&&(this.view.scrollDOM.insertBefore(this.dom,e),this.domAfter&&this.view.scrollDOM.appendChild(this.domAfter))}updateGutters(i){let e=i.startState.facet(_n),t=i.state.facet(_n),n=i.docChanged||i.heightChanged||i.viewportChanged||!H.eq(i.startState.facet(Li),i.state.facet(Li),i.view.viewport.from,i.view.viewport.to);if(e==t)for(let s of this.gutters)s.update(i)&&(n=!0);else{n=!0;let s=[];for(let r of t){let o=e.indexOf(r);o<0?s.push(new Ia(this.view,r)):(this.gutters[o].update(i),s.push(this.gutters[o]))}for(let r of this.gutters)r.dom.remove(),s.indexOf(r)<0&&r.destroy();for(let r of s)r.config.side=="after"?this.getDOMAfter().appendChild(r.dom):this.dom.appendChild(r.dom);this.gutters=s}return n}destroy(){for(let i of this.gutters)i.destroy();this.dom.remove(),this.domAfter&&this.domAfter.remove()}},{provide:i=>R.scrollMargins.of(e=>{let t=e.plugin(i);if(!t||t.gutters.length==0||!t.fixed)return null;let n=t.dom.offsetWidth*e.scaleX,s=t.domAfter?t.domAfter.offsetWidth*e.scaleX:0;return e.textDirection==J.LTR?{left:n,right:s}:{right:n,left:s}})});function Oa(i){return Array.isArray(i)?i:[i]}function Pr(i,e,t){for(;i.value&&i.from<=t;)i.from==t&&e.push(i.value),i.next()}class sp{constructor(e,t,n){this.gutter=e,this.height=n,this.i=0,this.cursor=H.iter(e.markers,t.from)}addElement(e,t,n){let{gutter:s}=this,r=(t.top-this.height)/e.scaleY,o=t.height/e.scaleY;if(this.i==s.elements.length){let a=new Hh(e,o,r,n);s.elements.push(a),s.dom.appendChild(a.dom)}else s.elements[this.i].update(e,o,r,n);this.height=t.bottom,this.i++}line(e,t,n){let s=[];Pr(this.cursor,s,t.from),n.length&&(s=s.concat(n));let r=this.gutter.config.lineMarker(e,t,s);r&&s.unshift(r);let o=this.gutter;s.length==0&&!o.config.renderEmptyElements||this.addElement(e,t,s)}widget(e,t){let n=this.gutter.config.widgetMarker(e,t.widget,t),s=n?[n]:null;for(let r of e.state.facet(ep)){let o=r(e,t.widget,t);o&&(s||(s=[])).push(o)}s&&this.addElement(e,t,s)}finish(){let e=this.gutter;for(;e.elements.length>this.i;){let t=e.elements.pop();e.dom.removeChild(t.dom),t.destroy()}}}class Ia{constructor(e,t){this.view=e,this.config=t,this.elements=[],this.spacer=null,this.dom=document.createElement("div"),this.dom.className="cm-gutter"+(this.config.class?" "+this.config.class:"");for(let n in t.domEventHandlers)this.dom.addEventListener(n,s=>{let r=s.target,o;if(r!=this.dom&&this.dom.contains(r)){for(;r.parentNode!=this.dom;)r=r.parentNode;let l=r.getBoundingClientRect();o=(l.top+l.bottom)/2}else o=s.clientY;let a=e.lineBlockAtHeight(o-e.documentTop);t.domEventHandlers[n](e,a,s)&&s.preventDefault()});this.markers=Oa(t.markers(e)),t.initialSpacer&&(this.spacer=new Hh(e,0,0,[t.initialSpacer(e)]),this.dom.appendChild(this.spacer.dom),this.spacer.dom.style.cssText+="visibility: hidden; pointer-events: none")}update(e){let t=this.markers;if(this.markers=Oa(this.config.markers(e.view)),this.spacer&&this.config.updateSpacer){let s=this.config.updateSpacer(this.spacer.markers[0],e);s!=this.spacer.markers[0]&&this.spacer.update(e.view,0,0,[s])}let n=e.view.viewport;return!H.eq(this.markers,t,n.from,n.to)||(this.config.lineMarkerChange?this.config.lineMarkerChange(e):!1)}destroy(){for(let e of this.elements)e.destroy()}}class Hh{constructor(e,t,n,s){this.height=-1,this.above=0,this.markers=[],this.dom=document.createElement("div"),this.dom.className="cm-gutterElement",this.update(e,t,n,s)}update(e,t,n,s){this.height!=t&&(this.height=t,this.dom.style.height=t+"px"),this.above!=n&&(this.dom.style.marginTop=(this.above=n)?n+"px":""),rp(this.markers,s)||this.setMarkers(e,s)}setMarkers(e,t){let n="cm-gutterElement",s=this.dom.firstChild;for(let r=0,o=0;;){let a=o,l=r<t.length?t[r++]:null,h=!1;if(l){let c=l.elementClass;c&&(n+=" "+c);for(let d=o;d<this.markers.length;d++)if(this.markers[d].compare(l)){a=d,h=!0;break}}else a=this.markers.length;for(;o<a;){let c=this.markers[o++];if(c.toDOM){c.destroy(s);let d=s.nextSibling;s.remove(),s=d}}if(!l)break;l.toDOM&&(h?s=s.nextSibling:this.dom.insertBefore(l.toDOM(e),s)),h&&o++}this.dom.className=n,this.markers=t}destroy(){this.setMarkers(null,[])}}function rp(i,e){if(i.length!=e.length)return!1;for(let t=0;t<i.length;t++)if(!i[t].compare(e[t]))return!1;return!0}const op=D.define(),ap=D.define(),Jt=D.define({combine(i){return bt(i,{formatNumber:String,domEventHandlers:{}},{domEventHandlers(e,t){let n=Object.assign({},e);for(let s in t){let r=n[s],o=t[s];n[s]=r?(a,l,h)=>r(a,l,h)||o(a,l,h):o}return n}})}});class Us extends gt{constructor(e){super(),this.number=e}eq(e){return this.number==e.number}toDOM(){return document.createTextNode(this.number)}}function $s(i,e){return i.state.facet(Jt).formatNumber(e,i.state)}const lp=_n.compute([Jt],i=>({class:"cm-lineNumbers",renderEmptyElements:!1,markers(e){return e.state.facet(op)},lineMarker(e,t,n){return n.some(s=>s.toDOM)?null:new Us($s(e,e.state.doc.lineAt(t.from).number))},widgetMarker:(e,t,n)=>{for(let s of e.state.facet(ap)){let r=s(e,t,n);if(r)return r}return null},lineMarkerChange:e=>e.startState.facet(Jt)!=e.state.facet(Jt),initialSpacer(e){return new Us($s(e,Da(e.state.doc.lines)))},updateSpacer(e,t){let n=$s(t.view,Da(t.view.state.doc.lines));return n==e.number?e:new Us(n)},domEventHandlers:i.facet(Jt).domEventHandlers,side:"before"}));function hp(i={}){return[Jt.of(i),qh(),lp]}function Da(i){let e=9;for(;e<i;)e=e*10+9;return e}const cp=new class extends gt{constructor(){super(...arguments),this.elementClass="cm-activeLineGutter"}},dp=Li.compute(["selection"],i=>{let e=[],t=-1;for(let n of i.selection.ranges){let s=i.doc.lineAt(n.head).from;s>t&&(t=s,e.push(cp.range(s)))}return H.of(e)});function up(){return dp}const fp=1024;let pp=0;class Ks{constructor(e,t){this.from=e,this.to=t}}class U{constructor(e={}){this.id=pp++,this.perNode=!!e.perNode,this.deserialize=e.deserialize||(()=>{throw new Error("This node type doesn't define a deserialize function")}),this.combine=e.combine||null}add(e){if(this.perNode)throw new RangeError("Can't add per-node props to node types");return typeof e!="function"&&(e=ze.match(e)),t=>{let n=e(t);return n===void 0?null:[this,n]}}}U.closedBy=new U({deserialize:i=>i.split(" ")});U.openedBy=new U({deserialize:i=>i.split(" ")});U.group=new U({deserialize:i=>i.split(" ")});U.isolate=new U({deserialize:i=>{if(i&&i!="rtl"&&i!="ltr"&&i!="auto")throw new RangeError("Invalid value for isolate: "+i);return i||"auto"}});U.contextHash=new U({perNode:!0});U.lookAhead=new U({perNode:!0});U.mounted=new U({perNode:!0});class Pn{constructor(e,t,n,s=!1){this.tree=e,this.overlay=t,this.parser=n,this.bracketed=s}static get(e){return e&&e.props&&e.props[U.mounted.id]}}const mp=Object.create(null);class ze{constructor(e,t,n,s=0){this.name=e,this.props=t,this.id=n,this.flags=s}static define(e){let t=e.props&&e.props.length?Object.create(null):mp,n=(e.top?1:0)|(e.skipped?2:0)|(e.error?4:0)|(e.name==null?8:0),s=new ze(e.name||"",t,e.id,n);if(e.props){for(let r of e.props)if(Array.isArray(r)||(r=r(s)),r){if(r[0].perNode)throw new RangeError("Can't store a per-node prop on a node type");t[r[0].id]=r[1]}}return s}prop(e){return this.props[e.id]}get isTop(){return(this.flags&1)>0}get isSkipped(){return(this.flags&2)>0}get isError(){return(this.flags&4)>0}get isAnonymous(){return(this.flags&8)>0}is(e){if(typeof e=="string"){if(this.name==e)return!0;let t=this.prop(U.group);return t?t.indexOf(e)>-1:!1}return this.id==e}static match(e){let t=Object.create(null);for(let n in e)for(let s of n.split(" "))t[s]=e[n];return n=>{for(let s=n.prop(U.group),r=-1;r<(s?s.length:0);r++){let o=t[r<0?n.name:s[r]];if(o)return o}}}}ze.none=new ze("",Object.create(null),0,8);const yi=new WeakMap,Ra=new WeakMap;var ie;(function(i){i[i.ExcludeBuffers=1]="ExcludeBuffers",i[i.IncludeAnonymous=2]="IncludeAnonymous",i[i.IgnoreMounts=4]="IgnoreMounts",i[i.IgnoreOverlays=8]="IgnoreOverlays",i[i.EnterBracketed=16]="EnterBracketed"})(ie||(ie={}));class fe{constructor(e,t,n,s,r){if(this.type=e,this.children=t,this.positions=n,this.length=s,this.props=null,r&&r.length){this.props=Object.create(null);for(let[o,a]of r)this.props[typeof o=="number"?o:o.id]=a}}toString(){let e=Pn.get(this);if(e&&!e.overlay)return e.tree.toString();let t="";for(let n of this.children){let s=n.toString();s&&(t&&(t+=","),t+=s)}return this.type.name?(/\W/.test(this.type.name)&&!this.type.isError?JSON.stringify(this.type.name):this.type.name)+(t.length?"("+t+")":""):t}cursor(e=0){return new Wr(this.topNode,e)}cursorAt(e,t=0,n=0){let s=yi.get(this)||this.topNode,r=new Wr(s);return r.moveTo(e,t),yi.set(this,r._tree),r}get topNode(){return new Ve(this,0,0,null)}resolve(e,t=0){let n=$n(yi.get(this)||this.topNode,e,t,!1);return yi.set(this,n),n}resolveInner(e,t=0){let n=$n(Ra.get(this)||this.topNode,e,t,!0);return Ra.set(this,n),n}resolveStack(e,t=0){return bp(this,e,t)}iterate(e){let{enter:t,leave:n,from:s=0,to:r=this.length}=e,o=e.mode||0,a=(o&ie.IncludeAnonymous)>0;for(let l=this.cursor(o|ie.IncludeAnonymous);;){let h=!1;if(l.from<=r&&l.to>=s&&(!a&&l.type.isAnonymous||t(l)!==!1)){if(l.firstChild())continue;h=!0}for(;h&&n&&(a||!l.type.isAnonymous)&&n(l),!l.nextSibling();){if(!l.parent())return;h=!0}}}prop(e){return e.perNode?this.props?this.props[e.id]:void 0:this.type.prop(e)}get propValues(){let e=[];if(this.props)for(let t in this.props)e.push([+t,this.props[t]]);return e}balance(e={}){return this.children.length<=8?this:mo(ze.none,this.children,this.positions,0,this.children.length,0,this.length,(t,n,s)=>new fe(this.type,t,n,s,this.propValues),e.makeTree||((t,n,s)=>new fe(ze.none,t,n,s)))}static build(e){return wp(e)}}fe.empty=new fe(ze.none,[],[],0);class fo{constructor(e,t){this.buffer=e,this.index=t}get id(){return this.buffer[this.index-4]}get start(){return this.buffer[this.index-3]}get end(){return this.buffer[this.index-2]}get size(){return this.buffer[this.index-1]}get pos(){return this.index}next(){this.index-=4}fork(){return new fo(this.buffer,this.index)}}class Dt{constructor(e,t,n){this.buffer=e,this.length=t,this.set=n}get type(){return ze.none}toString(){let e=[];for(let t=0;t<this.buffer.length;)e.push(this.childString(t)),t=this.buffer[t+3];return e.join(",")}childString(e){let t=this.buffer[e],n=this.buffer[e+3],s=this.set.types[t],r=s.name;if(/\W/.test(r)&&!s.isError&&(r=JSON.stringify(r)),e+=4,n==e)return r;let o=[];for(;e<n;)o.push(this.childString(e)),e=this.buffer[e+3];return r+"("+o.join(",")+")"}findChild(e,t,n,s,r){let{buffer:o}=this,a=-1;for(let l=e;l!=t&&!(Fh(r,s,o[l+1],o[l+2])&&(a=l,n>0));l=o[l+3]);return a}slice(e,t,n){let s=this.buffer,r=new Uint16Array(t-e),o=0;for(let a=e,l=0;a<t;){r[l++]=s[a++],r[l++]=s[a++]-n;let h=r[l++]=s[a++]-n;r[l++]=s[a++]-e,o=Math.max(o,h)}return new Dt(r,o,this.set)}}function Fh(i,e,t,n){switch(i){case-2:return t<e;case-1:return n>=e&&t<e;case 0:return t<e&&n>e;case 1:return t<=e&&n>e;case 2:return n>e;case 4:return!0}}function $n(i,e,t,n){for(var s;i.from==i.to||(t<1?i.from>=e:i.from>e)||(t>-1?i.to<=e:i.to<e);){let o=!n&&i instanceof Ve&&i.index<0?null:i.parent;if(!o)return i;i=o}let r=n?0:ie.IgnoreOverlays;if(n)for(let o=i,a=o.parent;a;o=a,a=o.parent)o instanceof Ve&&o.index<0&&((s=a.enter(e,t,r))===null||s===void 0?void 0:s.from)!=o.from&&(i=a);for(;;){let o=i.enter(e,t,r);if(!o)return i;i=o}}class Vh{cursor(e=0){return new Wr(this,e)}getChild(e,t=null,n=null){let s=Ea(this,e,t,n);return s.length?s[0]:null}getChildren(e,t=null,n=null){return Ea(this,e,t,n)}resolve(e,t=0){return $n(this,e,t,!1)}resolveInner(e,t=0){return $n(this,e,t,!0)}matchContext(e){return Nr(this.parent,e)}enterUnfinishedNodesBefore(e){let t=this.childBefore(e),n=this;for(;t;){let s=t.lastChild;if(!s||s.to!=t.to)break;s.type.isError&&s.from==s.to?(n=t,t=s.prevSibling):t=s}return n}get node(){return this}get next(){return this.parent}}class Ve extends Vh{constructor(e,t,n,s){super(),this._tree=e,this.from=t,this.index=n,this._parent=s}get type(){return this._tree.type}get name(){return this._tree.type.name}get to(){return this.from+this._tree.length}nextChild(e,t,n,s,r=0){for(let o=this;;){for(let{children:a,positions:l}=o._tree,h=t>0?a.length:-1;e!=h;e+=t){let c=a[e],d=l[e]+o.from,u;if(!(!(r&ie.EnterBracketed&&c instanceof fe&&(u=Pn.get(c))&&!u.overlay&&u.bracketed&&n>=d&&n<=d+c.length)&&!Fh(s,n,d,d+c.length))){if(c instanceof Dt){if(r&ie.ExcludeBuffers)continue;let f=c.findChild(0,c.buffer.length,t,n-d,s);if(f>-1)return new Ct(new gp(o,c,e,d),null,f)}else if(r&ie.IncludeAnonymous||!c.type.isAnonymous||po(c)){let f;if(!(r&ie.IgnoreMounts)&&(f=Pn.get(c))&&!f.overlay)return new Ve(f.tree,d,e,o);let p=new Ve(c,d,e,o);return r&ie.IncludeAnonymous||!p.type.isAnonymous?p:p.nextChild(t<0?c.children.length-1:0,t,n,s,r)}}}if(r&ie.IncludeAnonymous||!o.type.isAnonymous||(o.index>=0?e=o.index+t:e=t<0?-1:o._parent._tree.children.length,o=o._parent,!o))return null}}get firstChild(){return this.nextChild(0,1,0,4)}get lastChild(){return this.nextChild(this._tree.children.length-1,-1,0,4)}childAfter(e){return this.nextChild(0,1,e,2)}childBefore(e){return this.nextChild(this._tree.children.length-1,-1,e,-2)}prop(e){return this._tree.prop(e)}enter(e,t,n=0){let s;if(!(n&ie.IgnoreOverlays)&&(s=Pn.get(this._tree))&&s.overlay){let r=e-this.from,o=n&ie.EnterBracketed&&s.bracketed;for(let{from:a,to:l}of s.overlay)if((t>0||o?a<=r:a<r)&&(t<0||o?l>=r:l>r))return new Ve(s.tree,s.overlay[0].from+this.from,-1,this)}return this.nextChild(0,1,e,t,n)}nextSignificantParent(){let e=this;for(;e.type.isAnonymous&&e._parent;)e=e._parent;return e}get parent(){return this._parent?this._parent.nextSignificantParent():null}get nextSibling(){return this._parent&&this.index>=0?this._parent.nextChild(this.index+1,1,0,4):null}get prevSibling(){return this._parent&&this.index>=0?this._parent.nextChild(this.index-1,-1,0,4):null}get tree(){return this._tree}toTree(){return this._tree}toString(){return this._tree.toString()}}function Ea(i,e,t,n){let s=i.cursor(),r=[];if(!s.firstChild())return r;if(t!=null){for(let o=!1;!o;)if(o=s.type.is(t),!s.nextSibling())return r}for(;;){if(n!=null&&s.type.is(n))return r;if(s.type.is(e)&&r.push(s.node),!s.nextSibling())return n==null?r:[]}}function Nr(i,e,t=e.length-1){for(let n=i;t>=0;n=n.parent){if(!n)return!1;if(!n.type.isAnonymous){if(e[t]&&e[t]!=n.name)return!1;t--}}return!0}class gp{constructor(e,t,n,s){this.parent=e,this.buffer=t,this.index=n,this.start=s}}class Ct extends Vh{get name(){return this.type.name}get from(){return this.context.start+this.context.buffer.buffer[this.index+1]}get to(){return this.context.start+this.context.buffer.buffer[this.index+2]}constructor(e,t,n){super(),this.context=e,this._parent=t,this.index=n,this.type=e.buffer.set.types[e.buffer.buffer[n]]}child(e,t,n){let{buffer:s}=this.context,r=s.findChild(this.index+4,s.buffer[this.index+3],e,t-this.context.start,n);return r<0?null:new Ct(this.context,this,r)}get firstChild(){return this.child(1,0,4)}get lastChild(){return this.child(-1,0,4)}childAfter(e){return this.child(1,e,2)}childBefore(e){return this.child(-1,e,-2)}prop(e){return this.type.prop(e)}enter(e,t,n=0){if(n&ie.ExcludeBuffers)return null;let{buffer:s}=this.context,r=s.findChild(this.index+4,s.buffer[this.index+3],t>0?1:-1,e-this.context.start,t);return r<0?null:new Ct(this.context,this,r)}get parent(){return this._parent||this.context.parent.nextSignificantParent()}externalSibling(e){return this._parent?null:this.context.parent.nextChild(this.context.index+e,e,0,4)}get nextSibling(){let{buffer:e}=this.context,t=e.buffer[this.index+3];return t<(this._parent?e.buffer[this._parent.index+3]:e.buffer.length)?new Ct(this.context,this._parent,t):this.externalSibling(1)}get prevSibling(){let{buffer:e}=this.context,t=this._parent?this._parent.index+4:0;return this.index==t?this.externalSibling(-1):new Ct(this.context,this._parent,e.findChild(t,this.index,-1,0,4))}get tree(){return null}toTree(){let e=[],t=[],{buffer:n}=this.context,s=this.index+4,r=n.buffer[this.index+3];if(r>s){let o=n.buffer[this.index+1];e.push(n.slice(s,r,o)),t.push(0)}return new fe(this.type,e,t,this.to-this.from)}toString(){return this.context.buffer.childString(this.index)}}function zh(i){if(!i.length)return null;let e=0,t=i[0];for(let r=1;r<i.length;r++){let o=i[r];(o.from>t.from||o.to<t.to)&&(t=o,e=r)}let n=t instanceof Ve&&t.index<0?null:t.parent,s=i.slice();return n?s[e]=n:s.splice(e,1),new yp(s,t)}class yp{constructor(e,t){this.heads=e,this.node=t}get next(){return zh(this.heads)}}function bp(i,e,t){let n=i.resolveInner(e,t),s=null;for(let r=n instanceof Ve?n:n.context.parent;r;r=r.parent)if(r.index<0){let o=r.parent;(s||(s=[n])).push(o.resolve(e,t)),r=o}else{let o=Pn.get(r.tree);if(o&&o.overlay&&o.overlay[0].from<=e&&o.overlay[o.overlay.length-1].to>=e){let a=new Ve(o.tree,o.overlay[0].from+r.from,-1,r);(s||(s=[n])).push($n(a,e,t,!1))}}return s?zh(s):n}class Wr{get name(){return this.type.name}constructor(e,t=0){if(this.buffer=null,this.stack=[],this.index=0,this.bufferNode=null,this.mode=t&~ie.EnterBracketed,e instanceof Ve)this.yieldNode(e);else{this._tree=e.context.parent,this.buffer=e.context;for(let n=e._parent;n;n=n._parent)this.stack.unshift(n.index);this.bufferNode=e,this.yieldBuf(e.index)}}yieldNode(e){return e?(this._tree=e,this.type=e.type,this.from=e.from,this.to=e.to,!0):!1}yieldBuf(e,t){this.index=e;let{start:n,buffer:s}=this.buffer;return this.type=t||s.set.types[s.buffer[e]],this.from=n+s.buffer[e+1],this.to=n+s.buffer[e+2],!0}yield(e){return e?e instanceof Ve?(this.buffer=null,this.yieldNode(e)):(this.buffer=e.context,this.yieldBuf(e.index,e.type)):!1}toString(){return this.buffer?this.buffer.buffer.childString(this.index):this._tree.toString()}enterChild(e,t,n){if(!this.buffer)return this.yield(this._tree.nextChild(e<0?this._tree._tree.children.length-1:0,e,t,n,this.mode));let{buffer:s}=this.buffer,r=s.findChild(this.index+4,s.buffer[this.index+3],e,t-this.buffer.start,n);return r<0?!1:(this.stack.push(this.index),this.yieldBuf(r))}firstChild(){return this.enterChild(1,0,4)}lastChild(){return this.enterChild(-1,0,4)}childAfter(e){return this.enterChild(1,e,2)}childBefore(e){return this.enterChild(-1,e,-2)}enter(e,t,n=this.mode){return this.buffer?n&ie.ExcludeBuffers?!1:this.enterChild(1,e,t):this.yield(this._tree.enter(e,t,n))}parent(){if(!this.buffer)return this.yieldNode(this.mode&ie.IncludeAnonymous?this._tree._parent:this._tree.parent);if(this.stack.length)return this.yieldBuf(this.stack.pop());let e=this.mode&ie.IncludeAnonymous?this.buffer.parent:this.buffer.parent.nextSignificantParent();return this.buffer=null,this.yieldNode(e)}sibling(e){if(!this.buffer)return this._tree._parent?this.yield(this._tree.index<0?null:this._tree._parent.nextChild(this._tree.index+e,e,0,4,this.mode)):!1;let{buffer:t}=this.buffer,n=this.stack.length-1;if(e<0){let s=n<0?0:this.stack[n]+4;if(this.index!=s)return this.yieldBuf(t.findChild(s,this.index,-1,0,4))}else{let s=t.buffer[this.index+3];if(s<(n<0?t.buffer.length:t.buffer[this.stack[n]+3]))return this.yieldBuf(s)}return n<0?this.yield(this.buffer.parent.nextChild(this.buffer.index+e,e,0,4,this.mode)):!1}nextSibling(){return this.sibling(1)}prevSibling(){return this.sibling(-1)}atLastNode(e){let t,n,{buffer:s}=this;if(s){if(e>0){if(this.index<s.buffer.buffer.length)return!1}else for(let r=0;r<this.index;r++)if(s.buffer.buffer[r+3]<this.index)return!1;({index:t,parent:n}=s)}else({index:t,_parent:n}=this._tree);for(;n;{index:t,_parent:n}=n)if(t>-1)for(let r=t+e,o=e<0?-1:n._tree.children.length;r!=o;r+=e){let a=n._tree.children[r];if(this.mode&ie.IncludeAnonymous||a instanceof Dt||!a.type.isAnonymous||po(a))return!1}return!0}move(e,t){if(t&&this.enterChild(e,0,4))return!0;for(;;){if(this.sibling(e))return!0;if(this.atLastNode(e)||!this.parent())return!1}}next(e=!0){return this.move(1,e)}prev(e=!0){return this.move(-1,e)}moveTo(e,t=0){for(;(this.from==this.to||(t<1?this.from>=e:this.from>e)||(t>-1?this.to<=e:this.to<e))&&this.parent(););for(;this.enterChild(1,e,t););return this}get node(){if(!this.buffer)return this._tree;let e=this.bufferNode,t=null,n=0;if(e&&e.context==this.buffer)e:for(let s=this.index,r=this.stack.length;r>=0;){for(let o=e;o;o=o._parent)if(o.index==s){if(s==this.index)return o;t=o,n=r+1;break e}s=this.stack[--r]}for(let s=n;s<this.stack.length;s++)t=new Ct(this.buffer,t,this.stack[s]);return this.bufferNode=new Ct(this.buffer,t,this.index)}get tree(){return this.buffer?null:this._tree._tree}iterate(e,t){for(let n=0;;){let s=!1;if(this.type.isAnonymous||e(this)!==!1){if(this.firstChild()){n++;continue}this.type.isAnonymous||(s=!0)}for(;;){if(s&&t&&t(this),s=this.type.isAnonymous,!n)return;if(this.nextSibling())break;this.parent(),n--,s=!0}}}matchContext(e){if(!this.buffer)return Nr(this.node.parent,e);let{buffer:t}=this.buffer,{types:n}=t.set;for(let s=e.length-1,r=this.stack.length-1;s>=0;r--){if(r<0)return Nr(this._tree,e,s);let o=n[t.buffer[this.stack[r]]];if(!o.isAnonymous){if(e[s]&&e[s]!=o.name)return!1;s--}}return!0}}function po(i){return i.children.some(e=>e instanceof Dt||!e.type.isAnonymous||po(e))}function wp(i){var e;let{buffer:t,nodeSet:n,maxBufferLength:s=fp,reused:r=[],minRepeatType:o=n.types.length}=i,a=Array.isArray(t)?new fo(t,t.length):t,l=n.types,h=0,c=0;function d(S,C,A,L,q,F){let{id:W,start:E,end:$,size:V}=a,K=c,ee=h;if(V<0)if(a.next(),V==-1){let Ue=r[W];A.push(Ue),L.push(E-S);return}else if(V==-3){h=W;return}else if(V==-4){c=W;return}else throw new RangeError(`Unrecognized record size: ${V}`);let ke=l[W],Ee,re,Me=E-S;if($-E<=s&&(re=g(a.pos-C,q))){let Ue=new Uint16Array(re.size-re.skip),Oe=a.pos-re.size,$e=Ue.length;for(;a.pos>Oe;)$e=y(re.start,Ue,$e);Ee=new Dt(Ue,$-re.start,n),Me=re.start-S}else{let Ue=a.pos-V;a.next();let Oe=[],$e=[],ge=W>=o?W:-1,w=0,k=$;for(;a.pos>Ue;)ge>=0&&a.id==ge&&a.size>=0?(a.end<=k-s&&(p(Oe,$e,E,w,a.end,k,ge,K,ee),w=Oe.length,k=a.end),a.next()):F>2500?u(E,Ue,Oe,$e):d(E,Ue,Oe,$e,ge,F+1);if(ge>=0&&w>0&&w<Oe.length&&p(Oe,$e,E,w,E,k,ge,K,ee),Oe.reverse(),$e.reverse(),ge>-1&&w>0){let T=f(ke,ee);Ee=mo(ke,Oe,$e,0,Oe.length,0,$-E,T,T)}else Ee=m(ke,Oe,$e,$-E,K-$,ee)}A.push(Ee),L.push(Me)}function u(S,C,A,L){let q=[],F=0,W=-1;for(;a.pos>C;){let{id:E,start:$,end:V,size:K}=a;if(K>4)a.next();else{if(W>-1&&$<W)break;W<0&&(W=V-s),q.push(E,$,V),F++,a.next()}}if(F){let E=new Uint16Array(F*4),$=q[q.length-2];for(let V=q.length-3,K=0;V>=0;V-=3)E[K++]=q[V],E[K++]=q[V+1]-$,E[K++]=q[V+2]-$,E[K++]=K;A.push(new Dt(E,q[2]-$,n)),L.push($-S)}}function f(S,C){return(A,L,q)=>{let F=0,W=A.length-1,E,$;if(W>=0&&(E=A[W])instanceof fe){if(!W&&E.type==S&&E.length==q)return E;($=E.prop(U.lookAhead))&&(F=L[W]+E.length+$)}return m(S,A,L,q,F,C)}}function p(S,C,A,L,q,F,W,E,$){let V=[],K=[];for(;S.length>L;)V.push(S.pop()),K.push(C.pop()+A-q);S.push(m(n.types[W],V,K,F-q,E-F,$)),C.push(q-A)}function m(S,C,A,L,q,F,W){if(F){let E=[U.contextHash,F];W=W?[E].concat(W):[E]}if(q>25){let E=[U.lookAhead,q];W=W?[E].concat(W):[E]}return new fe(S,C,A,L,W)}function g(S,C){let A=a.fork(),L=0,q=0,F=0,W=A.end-s,E={size:0,start:0,skip:0};e:for(let $=A.pos-S;A.pos>$;){let V=A.size;if(A.id==C&&V>=0){E.size=L,E.start=q,E.skip=F,F+=4,L+=4,A.next();continue}let K=A.pos-V;if(V<0||K<$||A.start<W)break;let ee=A.id>=o?4:0,ke=A.start;for(A.next();A.pos>K;){if(A.size<0)if(A.size==-3||A.size==-4)ee+=4;else break e;else A.id>=o&&(ee+=4);A.next()}q=ke,L+=V,F+=ee}return(C<0||L==S)&&(E.size=L,E.start=q,E.skip=F),E.size>4?E:void 0}function y(S,C,A){let{id:L,start:q,end:F,size:W}=a;if(a.next(),W>=0&&L<o){let E=A;if(W>4){let $=a.pos-(W-4);for(;a.pos>$;)A=y(S,C,A)}C[--A]=E,C[--A]=F-S,C[--A]=q-S,C[--A]=L}else W==-3?h=L:W==-4&&(c=L);return A}let x=[],v=[];for(;a.pos>0;)d(i.start||0,i.bufferStart||0,x,v,-1,0);let I=(e=i.length)!==null&&e!==void 0?e:x.length?v[0]+x[0].length:0;return new fe(l[i.topID],x.reverse(),v.reverse(),I)}const La=new WeakMap;function Bi(i,e){if(!i.isAnonymous||e instanceof Dt||e.type!=i)return 1;let t=La.get(e);if(t==null){t=1;for(let n of e.children){if(n.type!=i||!(n instanceof fe)){t=1;break}t+=Bi(i,n)}La.set(e,t)}return t}function mo(i,e,t,n,s,r,o,a,l){let h=0;for(let p=n;p<s;p++)h+=Bi(i,e[p]);let c=Math.ceil(h*1.5/8),d=[],u=[];function f(p,m,g,y,x){for(let v=g;v<y;){let I=v,S=m[v],C=Bi(i,p[v]);for(v++;v<y;v++){let A=Bi(i,p[v]);if(C+A>=c)break;C+=A}if(v==I+1){if(C>c){let A=p[I];f(A.children,A.positions,0,A.children.length,m[I]+x);continue}d.push(p[I])}else{let A=m[v-1]+p[v-1].length-S;d.push(mo(i,p,m,I,v,S,A,null,l))}u.push(S+x-r)}}return f(e,t,n,s,0),(a||l)(d,u,o)}class Ht{constructor(e,t,n,s,r=!1,o=!1){this.from=e,this.to=t,this.tree=n,this.offset=s,this.open=(r?1:0)|(o?2:0)}get openStart(){return(this.open&1)>0}get openEnd(){return(this.open&2)>0}static addTree(e,t=[],n=!1){let s=[new Ht(0,e.length,e,0,!1,n)];for(let r of t)r.to>e.length&&s.push(r);return s}static applyChanges(e,t,n=128){if(!t.length)return e;let s=[],r=1,o=e.length?e[0]:null;for(let a=0,l=0,h=0;;a++){let c=a<t.length?t[a]:null,d=c?c.fromA:1e9;if(d-l>=n)for(;o&&o.from<d;){let u=o;if(l>=u.from||d<=u.to||h){let f=Math.max(u.from,l)-h,p=Math.min(u.to,d)-h;u=f>=p?null:new Ht(f,p,u.tree,u.offset+h,a>0,!!c)}if(u&&s.push(u),o.to>d)break;o=r<e.length?e[r++]:null}if(!c)break;l=c.toA,h=c.toA-c.toB}return s}}class vp{startParse(e,t,n){return typeof e=="string"&&(e=new kp(e)),n=n?n.length?n.map(s=>new Ks(s.from,s.to)):[new Ks(0,0)]:[new Ks(0,e.length)],this.createParse(e,t||[],n)}parse(e,t,n){let s=this.startParse(e,t,n);for(;;){let r=s.advance();if(r)return r}}}class kp{constructor(e){this.string=e}get length(){return this.string.length}chunk(e){return this.string.slice(e)}get lineChunks(){return!1}read(e,t){return this.string.slice(e,t)}}new U({perNode:!0});let xp=0;class We{constructor(e,t,n,s){this.name=e,this.set=t,this.base=n,this.modified=s,this.id=xp++}toString(){let{name:e}=this;for(let t of this.modified)t.name&&(e=`${t.name}(${e})`);return e}static define(e,t){let n=typeof e=="string"?e:"?";if(e instanceof We&&(t=e),t!=null&&t.base)throw new Error("Can not derive from a modified tag");let s=new We(n,[],null,[]);if(s.set.push(s),t)for(let r of t.set)s.set.push(r);return s}static defineModifier(e){let t=new Gi(e);return n=>n.modified.indexOf(t)>-1?n:Gi.get(n.base||n,n.modified.concat(t).sort((s,r)=>s.id-r.id))}}let Sp=0;class Gi{constructor(e){this.name=e,this.instances=[],this.id=Sp++}static get(e,t){if(!t.length)return e;let n=t[0].instances.find(a=>a.base==e&&Cp(t,a.modified));if(n)return n;let s=[],r=new We(e.name,s,e,t);for(let a of t)a.instances.push(r);let o=Ap(t);for(let a of e.set)if(!a.modified.length)for(let l of o)s.push(Gi.get(a,l));return r}}function Cp(i,e){return i.length==e.length&&i.every((t,n)=>t==e[n])}function Ap(i){let e=[[]];for(let t=0;t<i.length;t++)for(let n=0,s=e.length;n<s;n++)e.push(e[n].concat(i[t]));return e.sort((t,n)=>n.length-t.length)}function Tp(i){let e=Object.create(null);for(let t in i){let n=i[t];Array.isArray(n)||(n=[n]);for(let s of t.split(" "))if(s){let r=[],o=2,a=s;for(let d=0;;){if(a=="..."&&d>0&&d+3==s.length){o=1;break}let u=/^"(?:[^"\\]|\\.)*?"|[^\/!]+/.exec(a);if(!u)throw new RangeError("Invalid path: "+s);if(r.push(u[0]=="*"?"":u[0][0]=='"'?JSON.parse(u[0]):u[0]),d+=u[0].length,d==s.length)break;let f=s[d++];if(d==s.length&&f=="!"){o=0;break}if(f!="/")throw new RangeError("Invalid path: "+s);a=s.slice(d)}let l=r.length-1,h=r[l];if(!h)throw new RangeError("Invalid path: "+s);let c=new Yi(n,o,l>0?r.slice(0,l):null);e[h]=c.sort(e[h])}}return Mp.add(e)}const Mp=new U({combine(i,e){let t,n,s;for(;i||e;){if(!i||e&&i.depth<=e.depth?(s=e,e=e.next):(s=i,i=i.next),t&&t.mode==s.mode&&!s.context&&!t.context)continue;let r=new Yi(s.tags,s.mode,s.context);t?t.next=r:n=r,t=r}return n}});class Yi{constructor(e,t,n,s){this.tags=e,this.mode=t,this.context=n,this.next=s}get opaque(){return this.mode==0}get inherit(){return this.mode==1}sort(e){return!e||e.depth<this.depth?(this.next=e,this):(e.next=this.sort(e.next),e)}get depth(){return this.context?this.context.length:0}}Yi.empty=new Yi([],2,null);function Op(i,e){let t=Object.create(null);for(let r of i)if(!Array.isArray(r.tag))t[r.tag.id]=r.class;else for(let o of r.tag)t[o.id]=r.class;let{scope:n,all:s=null}={};return{style:r=>{let o=s;for(let a of r)for(let l of a.set){let h=t[l.id];if(h){o=o?o+" "+h:h;break}}return o},scope:n}}const M=We.define,bi=M(),wt=M(),Ba=M(wt),_a=M(wt),vt=M(),wi=M(vt),Gs=M(vt),it=M(),Lt=M(it),tt=M(),nt=M(),qr=M(),Cn=M(qr),vi=M(),P={comment:bi,lineComment:M(bi),blockComment:M(bi),docComment:M(bi),name:wt,variableName:M(wt),typeName:Ba,tagName:M(Ba),propertyName:_a,attributeName:M(_a),className:M(wt),labelName:M(wt),namespace:M(wt),macroName:M(wt),literal:vt,string:wi,docString:M(wi),character:M(wi),attributeValue:M(wi),number:Gs,integer:M(Gs),float:M(Gs),bool:M(vt),regexp:M(vt),escape:M(vt),color:M(vt),url:M(vt),keyword:tt,self:M(tt),null:M(tt),atom:M(tt),unit:M(tt),modifier:M(tt),operatorKeyword:M(tt),controlKeyword:M(tt),definitionKeyword:M(tt),moduleKeyword:M(tt),operator:nt,derefOperator:M(nt),arithmeticOperator:M(nt),logicOperator:M(nt),bitwiseOperator:M(nt),compareOperator:M(nt),updateOperator:M(nt),definitionOperator:M(nt),typeOperator:M(nt),controlOperator:M(nt),punctuation:qr,separator:M(qr),bracket:Cn,angleBracket:M(Cn),squareBracket:M(Cn),paren:M(Cn),brace:M(Cn),content:it,heading:Lt,heading1:M(Lt),heading2:M(Lt),heading3:M(Lt),heading4:M(Lt),heading5:M(Lt),heading6:M(Lt),contentSeparator:M(it),list:M(it),quote:M(it),emphasis:M(it),strong:M(it),link:M(it),monospace:M(it),strikethrough:M(it),inserted:M(),deleted:M(),changed:M(),invalid:M(),meta:vi,documentMeta:M(vi),annotation:M(vi),processingInstruction:M(vi),definition:We.defineModifier("definition"),constant:We.defineModifier("constant"),function:We.defineModifier("function"),standard:We.defineModifier("standard"),local:We.defineModifier("local"),special:We.defineModifier("special")};for(let i in P){let e=P[i];e instanceof We&&(e.name=i)}Op([{tag:P.link,class:"tok-link"},{tag:P.heading,class:"tok-heading"},{tag:P.emphasis,class:"tok-emphasis"},{tag:P.strong,class:"tok-strong"},{tag:P.keyword,class:"tok-keyword"},{tag:P.atom,class:"tok-atom"},{tag:P.bool,class:"tok-bool"},{tag:P.url,class:"tok-url"},{tag:P.labelName,class:"tok-labelName"},{tag:P.inserted,class:"tok-inserted"},{tag:P.deleted,class:"tok-deleted"},{tag:P.literal,class:"tok-literal"},{tag:P.string,class:"tok-string"},{tag:P.number,class:"tok-number"},{tag:[P.regexp,P.escape,P.special(P.string)],class:"tok-string2"},{tag:P.variableName,class:"tok-variableName"},{tag:P.local(P.variableName),class:"tok-variableName tok-local"},{tag:P.definition(P.variableName),class:"tok-variableName tok-definition"},{tag:P.special(P.variableName),class:"tok-variableName2"},{tag:P.definition(P.propertyName),class:"tok-propertyName tok-definition"},{tag:P.typeName,class:"tok-typeName"},{tag:P.namespace,class:"tok-namespace"},{tag:P.className,class:"tok-className"},{tag:P.macroName,class:"tok-macroName"},{tag:P.propertyName,class:"tok-propertyName"},{tag:P.operator,class:"tok-operator"},{tag:P.comment,class:"tok-comment"},{tag:P.meta,class:"tok-meta"},{tag:P.invalid,class:"tok-invalid"},{tag:P.punctuation,class:"tok-punctuation"}]);var Ys;const ki=new U,Ip=new U;class ft{constructor(e,t,n=[],s=""){this.data=e,this.name=s,z.prototype.hasOwnProperty("tree")||Object.defineProperty(z.prototype,"tree",{get(){return Je(this)}}),this.parser=t,this.extension=[un.of(this),z.languageData.of((r,o,a)=>{let l=Pa(r,o,a),h=l.type.prop(ki);if(!h)return[];let c=r.facet(h),d=l.type.prop(Ip);if(d){let u=l.resolve(o-l.from,a);for(let f of d)if(f.test(u,r)){let p=r.facet(f.facet);return f.type=="replace"?p:p.concat(c)}}return c})].concat(n)}isActiveAt(e,t,n=-1){return Pa(e,t,n).type.prop(ki)==this.data}findRegions(e){let t=e.facet(un);if((t==null?void 0:t.data)==this.data)return[{from:0,to:e.doc.length}];if(!t||!t.allowsNesting)return[];let n=[],s=(r,o)=>{if(r.prop(ki)==this.data){n.push({from:o,to:o+r.length});return}let a=r.prop(U.mounted);if(a){if(a.tree.prop(ki)==this.data){if(a.overlay)for(let l of a.overlay)n.push({from:l.from+o,to:l.to+o});else n.push({from:o,to:o+r.length});return}else if(a.overlay){let l=n.length;if(s(a.tree,a.overlay[0].from+o),n.length>l)return}}for(let l=0;l<r.children.length;l++){let h=r.children[l];h instanceof fe&&s(h,r.positions[l]+o)}};return s(Je(e),0),n}get allowsNesting(){return!0}}ft.setState=N.define();function Pa(i,e,t){let n=i.facet(un),s=Je(i).topNode;if(!n||n.allowsNesting)for(let r=s;r;r=r.enter(e,t,ie.ExcludeBuffers|ie.EnterBracketed))r.type.isTop&&(s=r);return s}function Je(i){let e=i.field(ft.state,!1);return e?e.tree:fe.empty}class Dp{constructor(e){this.doc=e,this.cursorPos=0,this.string="",this.cursor=e.iter()}get length(){return this.doc.length}syncTo(e){return this.string=this.cursor.next(e-this.cursorPos).value,this.cursorPos=e+this.string.length,this.cursorPos-this.string.length}chunk(e){return this.syncTo(e),this.string}get lineChunks(){return!0}read(e,t){let n=this.cursorPos-this.string.length;return e<n||t>=this.cursorPos?this.doc.sliceString(e,t):this.string.slice(e-n,t-n)}}let An=null;class Ji{constructor(e,t,n=[],s,r,o,a,l){this.parser=e,this.state=t,this.fragments=n,this.tree=s,this.treeLen=r,this.viewport=o,this.skipped=a,this.scheduleOn=l,this.parse=null,this.tempSkipped=[]}static create(e,t,n){return new Ji(e,t,[],fe.empty,0,n,[],null)}startParse(){return this.parser.startParse(new Dp(this.state.doc),this.fragments)}work(e,t){return t!=null&&t>=this.state.doc.length&&(t=void 0),this.tree!=fe.empty&&this.isDone(t??this.state.doc.length)?(this.takeTree(),!0):this.withContext(()=>{var n;if(typeof e=="number"){let s=Date.now()+e;e=()=>Date.now()>s}for(this.parse||(this.parse=this.startParse()),t!=null&&(this.parse.stoppedAt==null||this.parse.stoppedAt>t)&&t<this.state.doc.length&&this.parse.stopAt(t);;){let s=this.parse.advance();if(s)if(this.fragments=this.withoutTempSkipped(Ht.addTree(s,this.fragments,this.parse.stoppedAt!=null)),this.treeLen=(n=this.parse.stoppedAt)!==null&&n!==void 0?n:this.state.doc.length,this.tree=s,this.parse=null,this.treeLen<(t??this.state.doc.length))this.parse=this.startParse();else return!0;if(e())return!1}})}takeTree(){let e,t;this.parse&&(e=this.parse.parsedPos)>=this.treeLen&&((this.parse.stoppedAt==null||this.parse.stoppedAt>e)&&this.parse.stopAt(e),this.withContext(()=>{for(;!(t=this.parse.advance()););}),this.treeLen=e,this.tree=t,this.fragments=this.withoutTempSkipped(Ht.addTree(this.tree,this.fragments,!0)),this.parse=null)}withContext(e){let t=An;An=this;try{return e()}finally{An=t}}withoutTempSkipped(e){for(let t;t=this.tempSkipped.pop();)e=Na(e,t.from,t.to);return e}changes(e,t){let{fragments:n,tree:s,treeLen:r,viewport:o,skipped:a}=this;if(this.takeTree(),!e.empty){let l=[];if(e.iterChangedRanges((h,c,d,u)=>l.push({fromA:h,toA:c,fromB:d,toB:u})),n=Ht.applyChanges(n,l),s=fe.empty,r=0,o={from:e.mapPos(o.from,-1),to:e.mapPos(o.to,1)},this.skipped.length){a=[];for(let h of this.skipped){let c=e.mapPos(h.from,1),d=e.mapPos(h.to,-1);c<d&&a.push({from:c,to:d})}}}return new Ji(this.parser,t,n,s,r,o,a,this.scheduleOn)}updateViewport(e){if(this.viewport.from==e.from&&this.viewport.to==e.to)return!1;this.viewport=e;let t=this.skipped.length;for(let n=0;n<this.skipped.length;n++){let{from:s,to:r}=this.skipped[n];s<e.to&&r>e.from&&(this.fragments=Na(this.fragments,s,r),this.skipped.splice(n--,1))}return this.skipped.length>=t?!1:(this.reset(),!0)}reset(){this.parse&&(this.takeTree(),this.parse=null)}skipUntilInView(e,t){this.skipped.push({from:e,to:t})}static getSkippingParser(e){return new class extends vp{createParse(t,n,s){let r=s[0].from,o=s[s.length-1].to;return{parsedPos:r,advance(){let l=An;if(l){for(let h of s)l.tempSkipped.push(h);e&&(l.scheduleOn=l.scheduleOn?Promise.all([l.scheduleOn,e]):e)}return this.parsedPos=o,new fe(ze.none,[],[],o-r)},stoppedAt:null,stopAt(){}}}}}isDone(e){e=Math.min(e,this.state.doc.length);let t=this.fragments;return this.treeLen>=e&&t.length&&t[0].from==0&&t[0].to>=e}static get(){return An}}function Na(i,e,t){return Ht.applyChanges(i,[{fromA:e,toA:t,fromB:e,toB:t}])}class dn{constructor(e){this.context=e,this.tree=e.tree}apply(e){if(!e.docChanged&&this.tree==this.context.tree)return this;let t=this.context.changes(e.changes,e.state),n=this.context.treeLen==e.startState.doc.length?void 0:Math.max(e.changes.mapPos(this.context.treeLen),t.viewport.to);return t.work(20,n)||t.takeTree(),new dn(t)}static init(e){let t=Math.min(3e3,e.doc.length),n=Ji.create(e.facet(un).parser,e,{from:0,to:t});return n.work(20,t)||n.takeTree(),new dn(n)}}ft.state=me.define({create:dn.init,update(i,e){for(let t of e.effects)if(t.is(ft.setState))return t.value;return e.startState.facet(un)!=e.state.facet(un)?dn.init(e.state):i.apply(e)}});let jh=i=>{let e=setTimeout(()=>i(),500);return()=>clearTimeout(e)};typeof requestIdleCallback<"u"&&(jh=i=>{let e=-1,t=setTimeout(()=>{e=requestIdleCallback(i,{timeout:400})},100);return()=>e<0?clearTimeout(t):cancelIdleCallback(e)});const Js=typeof navigator<"u"&&(!((Ys=navigator.scheduling)===null||Ys===void 0)&&Ys.isInputPending)?()=>navigator.scheduling.isInputPending():null,Rp=ce.fromClass(class{constructor(e){this.view=e,this.working=null,this.workScheduled=0,this.chunkEnd=-1,this.chunkBudget=-1,this.work=this.work.bind(this),this.scheduleWork()}update(e){let t=this.view.state.field(ft.state).context;(t.updateViewport(e.view.viewport)||this.view.viewport.to>t.treeLen)&&this.scheduleWork(),(e.docChanged||e.selectionSet)&&(this.view.hasFocus&&(this.chunkBudget+=50),this.scheduleWork()),this.checkAsyncSchedule(t)}scheduleWork(){if(this.working)return;let{state:e}=this.view,t=e.field(ft.state);(t.tree!=t.context.tree||!t.context.isDone(e.doc.length))&&(this.working=jh(this.work))}work(e){this.working=null;let t=Date.now();if(this.chunkEnd<t&&(this.chunkEnd<0||this.view.hasFocus)&&(this.chunkEnd=t+3e4,this.chunkBudget=3e3),this.chunkBudget<=0)return;let{state:n,viewport:{to:s}}=this.view,r=n.field(ft.state);if(r.tree==r.context.tree&&r.context.isDone(s+1e5))return;let o=Date.now()+Math.min(this.chunkBudget,100,e&&!Js?Math.max(25,e.timeRemaining()-5):1e9),a=r.context.treeLen<s&&n.doc.length>s+1e3,l=r.context.work(()=>Js&&Js()||Date.now()>o,s+(a?0:1e5));this.chunkBudget-=Date.now()-t,(l||this.chunkBudget<=0)&&(r.context.takeTree(),this.view.dispatch({effects:ft.setState.of(new dn(r.context))})),this.chunkBudget>0&&!(l&&!a)&&this.scheduleWork(),this.checkAsyncSchedule(r.context)}checkAsyncSchedule(e){e.scheduleOn&&(this.workScheduled++,e.scheduleOn.then(()=>this.scheduleWork()).catch(t=>Ce(this.view.state,t)).then(()=>this.workScheduled--),e.scheduleOn=null)}destroy(){this.working&&this.working()}isWorking(){return!!(this.working||this.workScheduled>0)}},{eventHandlers:{focus(){this.scheduleWork()}}}),un=D.define({combine(i){return i.length?i[0]:null},enables:i=>[ft.state,Rp,R.contentAttributes.compute([i],e=>{let t=e.facet(i);return t&&t.name?{"data-language":t.name}:{}})]}),Ep=D.define(),bs=D.define({combine:i=>{if(!i.length)return"  ";let e=i[0];if(!e||/\S/.test(e)||Array.from(e).some(t=>t!=e[0]))throw new Error("Invalid indent unit: "+JSON.stringify(i[0]));return e}});function Xi(i){let e=i.facet(bs);return e.charCodeAt(0)==9?i.tabSize*e.length:e.length}function Qi(i,e){let t="",n=i.tabSize,s=i.facet(bs)[0];if(s=="	"){for(;e>=n;)t+="	",e-=n;s=" "}for(let r=0;r<e;r++)t+=s;return t}function Uh(i,e){i instanceof z&&(i=new ws(i));for(let n of i.state.facet(Ep)){let s=n(i,e);if(s!==void 0)return s}let t=Je(i.state);return t.length>=e?Bp(i,t,e):null}class ws{constructor(e,t={}){this.state=e,this.options=t,this.unit=Xi(e)}lineAt(e,t=1){let n=this.state.doc.lineAt(e),{simulateBreak:s,simulateDoubleBreak:r}=this.options;return s!=null&&s>=n.from&&s<=n.to?r&&s==e?{text:"",from:e}:(t<0?s<e:s<=e)?{text:n.text.slice(s-n.from),from:s}:{text:n.text.slice(0,s-n.from),from:n.from}:n}textAfterPos(e,t=1){if(this.options.simulateDoubleBreak&&e==this.options.simulateBreak)return"";let{text:n,from:s}=this.lineAt(e,t);return n.slice(e-s,Math.min(n.length,e+100-s))}column(e,t=1){let{text:n,from:s}=this.lineAt(e,t),r=this.countColumn(n,e-s),o=this.options.overrideIndentation?this.options.overrideIndentation(s):-1;return o>-1&&(r+=o-this.countColumn(n,n.search(/\S|$/))),r}countColumn(e,t=e.length){return hs(e,this.state.tabSize,t)}lineIndent(e,t=1){let{text:n,from:s}=this.lineAt(e,t),r=this.options.overrideIndentation;if(r){let o=r(s);if(o>-1)return o}return this.countColumn(n,n.search(/\S|$/))}get simulatedBreak(){return this.options.simulateBreak||null}}const Lp=new U;function Bp(i,e,t){let n=e.resolveStack(t),s=e.resolveInner(t,-1).resolve(t,0).enterUnfinishedNodesBefore(t);if(s!=n.node){let r=[];for(let o=s;o&&!(o.from<n.node.from||o.to>n.node.to||o.from==n.node.from&&o.type==n.node.type);o=o.parent)r.push(o);for(let o=r.length-1;o>=0;o--)n={node:r[o],next:n}}return $h(n,i,t)}function $h(i,e,t){for(let n=i;n;n=n.next){let s=Pp(n.node);if(s)return s(go.create(e,t,n))}return 0}function _p(i){return i.pos==i.options.simulateBreak&&i.options.simulateDoubleBreak}function Pp(i){let e=i.type.prop(Lp);if(e)return e;let t=i.firstChild,n;if(t&&(n=t.type.prop(U.closedBy))){let s=i.lastChild,r=s&&n.indexOf(s.name)>-1;return o=>Hp(o,!0,1,void 0,r&&!_p(o)?s.from:void 0)}return i.parent==null?Np:null}function Np(){return 0}class go extends ws{constructor(e,t,n){super(e.state,e.options),this.base=e,this.pos=t,this.context=n}get node(){return this.context.node}static create(e,t,n){return new go(e,t,n)}get textAfter(){return this.textAfterPos(this.pos)}get baseIndent(){return this.baseIndentFor(this.node)}baseIndentFor(e){let t=this.state.doc.lineAt(e.from);for(;;){let n=e.resolve(t.from);for(;n.parent&&n.parent.from==n.from;)n=n.parent;if(Wp(n,e))break;t=this.state.doc.lineAt(n.from)}return this.lineIndent(t.from)}continue(){return $h(this.context.next,this.base,this.pos)}}function Wp(i,e){for(let t=e;t;t=t.parent)if(i==t)return!0;return!1}function qp(i){let e=i.node,t=e.childAfter(e.from),n=e.lastChild;if(!t)return null;let s=i.options.simulateBreak,r=i.state.doc.lineAt(t.from),o=s==null||s<=r.from?r.to:Math.min(r.to,s);for(let a=t.to;;){let l=e.childAfter(a);if(!l||l==n)return null;if(!l.type.isSkipped){if(l.from>=o)return null;let h=/^ */.exec(r.text.slice(t.to-r.from))[0].length;return{from:t.from,to:t.to+h}}a=l.to}}function Hp(i,e,t,n,s){let r=i.textAfter,o=r.match(/^\s*/)[0].length,a=n&&r.slice(o,o+n.length)==n||s==i.pos+o,l=qp(i);return l?a?i.column(l.from):i.column(l.to):i.baseIndent+(a?0:i.unit*t)}const Fp=R.baseTheme({"&.cm-focused .cm-matchingBracket":{backgroundColor:"#328c8252"},"&.cm-focused .cm-nonmatchingBracket":{backgroundColor:"#bb555544"}}),Kh=1e4,Gh="()[]{}",Yh=D.define({combine(i){return bt(i,{afterCursor:!0,brackets:Gh,maxScanDistance:Kh,renderMatch:jp})}}),Vp=_.mark({class:"cm-matchingBracket"}),zp=_.mark({class:"cm-nonmatchingBracket"});function jp(i){let e=[],t=i.matched?Vp:zp;return e.push(t.range(i.start.from,i.start.to)),i.end&&e.push(t.range(i.end.from,i.end.to)),e}function Wa(i){let e=[],t=i.facet(Yh);for(let n of i.selection.ranges){if(!n.empty)continue;let s=lt(i,n.head,-1,t)||n.head>0&&lt(i,n.head-1,1,t)||t.afterCursor&&(lt(i,n.head,1,t)||n.head<i.doc.length&&lt(i,n.head+1,-1,t));s&&(e=e.concat(t.renderMatch(s,i)))}return _.set(e,!0)}const Up=ce.fromClass(class{constructor(i){this.paused=!1,this.decorations=Wa(i.state)}update(i){(i.docChanged||i.selectionSet||this.paused)&&(i.view.composing?(this.decorations=this.decorations.map(i.changes),this.paused=!0):(this.decorations=Wa(i.state),this.paused=!1))}},{decorations:i=>i.decorations}),$p=[Up,Fp];function Kp(i={}){return[Yh.of(i),$p]}const Gp=new U;function Hr(i,e,t){let n=i.prop(e<0?U.openedBy:U.closedBy);if(n)return n;if(i.name.length==1){let s=t.indexOf(i.name);if(s>-1&&s%2==(e<0?1:0))return[t[s+e]]}return null}function Fr(i){let e=i.type.prop(Gp);return e?e(i.node):i}function lt(i,e,t,n={}){let s=n.maxScanDistance||Kh,r=n.brackets||Gh,o=Je(i),a=o.resolveInner(e,t);for(let l=a;l;l=l.parent){let h=Hr(l.type,t,r);if(h&&l.from<l.to){let c=Fr(l);if(c&&(t>0?e>=c.from&&e<c.to:e>c.from&&e<=c.to))return Yp(i,e,t,l,c,h,r)}}return Jp(i,e,t,o,a.type,s,r)}function Yp(i,e,t,n,s,r,o){let a=n.parent,l={from:s.from,to:s.to},h=0,c=a==null?void 0:a.cursor();if(c&&(t<0?c.childBefore(n.from):c.childAfter(n.to)))do if(t<0?c.to<=n.from:c.from>=n.to){if(h==0&&r.indexOf(c.type.name)>-1&&c.from<c.to){let d=Fr(c);return{start:l,end:d?{from:d.from,to:d.to}:void 0,matched:!0}}else if(Hr(c.type,t,o))h++;else if(Hr(c.type,-t,o)){if(h==0){let d=Fr(c);return{start:l,end:d&&d.from<d.to?{from:d.from,to:d.to}:void 0,matched:!1}}h--}}while(t<0?c.prevSibling():c.nextSibling());return{start:l,matched:!1}}function Jp(i,e,t,n,s,r,o){if(t<0?!e:e==i.doc.length)return null;let a=t<0?i.sliceDoc(e-1,e):i.sliceDoc(e,e+1),l=o.indexOf(a);if(l<0||l%2==0!=t>0)return null;let h={from:t<0?e-1:e,to:t>0?e+1:e},c=i.doc.iterRange(e,t>0?i.doc.length:0),d=0;for(let u=0;!c.next().done&&u<=r;){let f=c.value;t<0&&(u+=f.length);let p=e+u*t;for(let m=t>0?0:f.length-1,g=t>0?f.length:-1;m!=g;m+=t){let y=o.indexOf(f[m]);if(!(y<0||n.resolveInner(p+m,1).type!=s))if(y%2==0==t>0)d++;else{if(d==1)return{start:h,end:{from:p+m,to:p+m+1},matched:y>>1==l>>1};d--}}t>0&&(u+=f.length)}return c.done?{start:h,matched:!1}:null}const Xp=Object.create(null),qa=[ze.none],Ha=[],Fa=Object.create(null),Qp=Object.create(null);for(let[i,e]of[["variable","variableName"],["variable-2","variableName.special"],["string-2","string.special"],["def","variableName.definition"],["tag","tagName"],["attribute","attributeName"],["type","typeName"],["builtin","variableName.standard"],["qualifier","modifier"],["error","invalid"],["header","heading"],["property","propertyName"]])Qp[i]=Zp(Xp,e);function Xs(i,e){Ha.indexOf(i)>-1||(Ha.push(i),console.warn(e))}function Zp(i,e){let t=[];for(let a of e.split(" ")){let l=[];for(let h of a.split(".")){let c=i[h]||P[h];c?typeof c=="function"?l.length?l=l.map(c):Xs(h,`Modifier ${h} used at start of tag`):l.length?Xs(h,`Tag ${h} used as modifier`):l=Array.isArray(c)?c:[c]:Xs(h,`Unknown highlighting tag ${h}`)}for(let h of l)t.push(h)}if(!t.length)return 0;let n=e.replace(/ /g,"_"),s=n+" "+t.map(a=>a.id),r=Fa[s];if(r)return r.id;let o=Fa[s]=ze.define({id:qa.length,name:n,props:[Tp({[n]:t})]});return qa.push(o),o.id}J.RTL,J.LTR;class Jh{constructor(e,t,n,s){this.state=e,this.pos=t,this.explicit=n,this.view=s,this.abortListeners=[],this.abortOnDocChange=!1}tokenBefore(e){let t=Je(this.state).resolveInner(this.pos,-1);for(;t&&e.indexOf(t.name)<0;)t=t.parent;return t?{from:t.from,to:this.pos,text:this.state.sliceDoc(t.from,this.pos),type:t.type}:null}matchBefore(e){let t=this.state.doc.lineAt(this.pos),n=Math.max(t.from,this.pos-250),s=t.text.slice(n-t.from,this.pos-t.from),r=s.search(Xh(e,!1));return r<0?null:{from:n+r,to:this.pos,text:s.slice(r)}}get aborted(){return this.abortListeners==null}addEventListener(e,t,n){e=="abort"&&this.abortListeners&&(this.abortListeners.push(t),n&&n.onDocChange&&(this.abortOnDocChange=!0))}}function Va(i){let e=Object.keys(i).join(""),t=/\w/.test(e);return t&&(e=e.replace(/\w/g,"")),`[${t?"\\w":""}${e.replace(/[^\w\s]/g,"\\$&")}]`}function em(i){let e=Object.create(null),t=Object.create(null);for(let{label:s}of i){e[s[0]]=!0;for(let r=1;r<s.length;r++)t[s[r]]=!0}let n=Va(e)+Va(t)+"*$";return[new RegExp("^"+n),new RegExp(n)]}function tm(i){let e=i.map(s=>typeof s=="string"?{label:s}:s),[t,n]=e.every(s=>/^\w+$/.test(s.label))?[/\w*$/,/\w+$/]:em(e);return s=>{let r=s.matchBefore(n);return r||s.explicit?{from:r?r.from:s.pos,options:e,validFor:t}:null}}class za{constructor(e,t,n,s){this.completion=e,this.source=t,this.match=n,this.score=s}}function Ft(i){return i.selection.main.from}function Xh(i,e){var t;let{source:n}=i,s=e&&n[0]!="^",r=n[n.length-1]!="$";return!s&&!r?i:new RegExp(`${s?"^":""}(?:${n})${r?"$":""}`,(t=i.flags)!==null&&t!==void 0?t:i.ignoreCase?"i":"")}const Qh=yt.define();function nm(i,e,t,n){let{main:s}=i.selection,r=t-s.from,o=n-s.from;return{...i.changeByRange(a=>{if(a!=s&&t!=n&&i.sliceDoc(a.from+r,a.from+o)!=i.sliceDoc(t,n))return{range:a};let l=i.toText(e);return{changes:{from:a.from+r,to:n==s.from?a.to:a.from+o,insert:l},range:b.cursor(a.from+r+l.length)}}),scrollIntoView:!0,userEvent:"input.complete"}}const ja=new WeakMap;function im(i){if(!Array.isArray(i))return i;let e=ja.get(i);return e||ja.set(i,e=tm(i)),e}const Zi=N.define(),Kn=N.define();class sm{constructor(e){this.pattern=e,this.chars=[],this.folded=[],this.any=[],this.precise=[],this.byWord=[],this.score=0,this.matched=[];for(let t=0;t<e.length;){let n=_e(e,t),s=rt(n);this.chars.push(n);let r=e.slice(t,t+s),o=r.toUpperCase();this.folded.push(_e(o==r?r.toLowerCase():o,0)),t+=s}this.astral=e.length!=this.chars.length}ret(e,t){return this.score=e,this.matched=t,this}match(e){if(this.pattern.length==0)return this.ret(-100,[]);if(e.length<this.pattern.length)return null;let{chars:t,folded:n,any:s,precise:r,byWord:o}=this;if(t.length==1){let x=_e(e,0),v=rt(x),I=v==e.length?0:-100;if(x!=t[0])if(x==n[0])I+=-200;else return null;return this.ret(I,[0,v])}let a=e.indexOf(this.pattern);if(a==0)return this.ret(e.length==this.pattern.length?0:-100,[0,this.pattern.length]);let l=t.length,h=0;if(a<0){for(let x=0,v=Math.min(e.length,200);x<v&&h<l;){let I=_e(e,x);(I==t[h]||I==n[h])&&(s[h++]=x),x+=rt(I)}if(h<l)return null}let c=0,d=0,u=!1,f=0,p=-1,m=-1,g=/[a-z]/.test(e),y=!0;for(let x=0,v=Math.min(e.length,200),I=0;x<v&&d<l;){let S=_e(e,x);a<0&&(c<l&&S==t[c]&&(r[c++]=x),f<l&&(S==t[f]||S==n[f]?(f==0&&(p=x),m=x+1,f++):f=0));let C,A=S<255?S>=48&&S<=57||S>=97&&S<=122?2:S>=65&&S<=90?1:0:(C=Gr(S))!=C.toLowerCase()?1:C!=C.toUpperCase()?2:0;(!x||A==1&&g||I==0&&A!=0)&&(t[d]==S||n[d]==S&&(u=!0)?o[d++]=x:o.length&&(y=!1)),I=A,x+=rt(S)}return d==l&&o[0]==0&&y?this.result(-100+(u?-200:0),o,e):f==l&&p==0?this.ret(-200-e.length+(m==e.length?0:-100),[0,m]):a>-1?this.ret(-700-e.length,[a,a+this.pattern.length]):f==l?this.ret(-900-e.length,[p,m]):d==l?this.result(-100+(u?-200:0)+-700+(y?0:-1100),o,e):t.length==2?null:this.result((s[0]?-700:0)+-200+-1100,s,e)}result(e,t,n){let s=[],r=0;for(let o of t){let a=o+(this.astral?rt(_e(n,o)):1);r&&s[r-1]==o?s[r-1]=a:(s[r++]=o,s[r++]=a)}return this.ret(e-n.length,s)}}class rm{constructor(e){this.pattern=e,this.matched=[],this.score=0,this.folded=e.toLowerCase()}match(e){if(e.length<this.pattern.length)return null;let t=e.slice(0,this.pattern.length),n=t==this.pattern?0:t.toLowerCase()==this.folded?-200:null;return n==null?null:(this.matched=[0,t.length],this.score=n+(e.length==this.pattern.length?0:-100),this)}}const le=D.define({combine(i){return bt(i,{activateOnTyping:!0,activateOnCompletion:()=>!1,activateOnTypingDelay:100,selectOnOpen:!0,override:null,closeOnBlur:!0,maxRenderedOptions:100,defaultKeymap:!0,tooltipClass:()=>"",optionClass:()=>"",aboveCursor:!1,icons:!0,addToOptions:[],positionInfo:om,filterStrict:!1,compareCompletions:(e,t)=>(e.sortText||e.label).localeCompare(t.sortText||t.label),interactionDelay:75,updateSyncTime:100},{defaultKeymap:(e,t)=>e&&t,closeOnBlur:(e,t)=>e&&t,icons:(e,t)=>e&&t,tooltipClass:(e,t)=>n=>Ua(e(n),t(n)),optionClass:(e,t)=>n=>Ua(e(n),t(n)),addToOptions:(e,t)=>e.concat(t),filterStrict:(e,t)=>e||t})}});function Ua(i,e){return i?e?i+" "+e:i:e}function om(i,e,t,n,s,r){let o=i.textDirection==J.RTL,a=o,l=!1,h="top",c,d,u=e.left-s.left,f=s.right-e.right,p=n.right-n.left,m=n.bottom-n.top;if(a&&u<Math.min(p,f)?a=!1:!a&&f<Math.min(p,u)&&(a=!0),p<=(a?u:f))c=Math.max(s.top,Math.min(t.top,s.bottom-m))-e.top,d=Math.min(400,a?u:f);else{l=!0,d=Math.min(400,(o?e.right:s.right-e.left)-30);let x=s.bottom-e.bottom;x>=m||x>e.top?c=t.bottom-e.top:(h="bottom",c=e.bottom-t.top)}let g=(e.bottom-e.top)/r.offsetHeight,y=(e.right-e.left)/r.offsetWidth;return{style:`${h}: ${c/g}px; max-width: ${d/y}px`,class:"cm-completionInfo-"+(l?o?"left-narrow":"right-narrow":a?"left":"right")}}const yo=N.define();function am(i){let e=i.addToOptions.slice();return i.icons&&e.push({render(t){let n=document.createElement("div");return n.classList.add("cm-completionIcon"),t.type&&n.classList.add(...t.type.split(/\s+/g).map(s=>"cm-completionIcon-"+s)),n.setAttribute("aria-hidden","true"),n},position:20}),e.push({render(t,n,s,r){let o=document.createElement("span");o.className="cm-completionLabel";let a=t.displayLabel||t.label,l=0;for(let h=0;h<r.length;){let c=r[h++],d=r[h++];c>l&&o.appendChild(document.createTextNode(a.slice(l,c)));let u=o.appendChild(document.createElement("span"));u.appendChild(document.createTextNode(a.slice(c,d))),u.className="cm-completionMatchedText",l=d}return l<a.length&&o.appendChild(document.createTextNode(a.slice(l))),o},position:50},{render(t){if(!t.detail)return null;let n=document.createElement("span");return n.className="cm-completionDetail",n.textContent=t.detail,n},position:80}),e.sort((t,n)=>t.position-n.position).map(t=>t.render)}function Qs(i,e,t){if(i<=t)return{from:0,to:i};if(e<0&&(e=0),e<=i>>1){let s=Math.floor(e/t);return{from:s*t,to:(s+1)*t}}let n=Math.ceil((i-e)/t);return{from:i-n*t,to:i-(n-1)*t}}class lm{constructor(e,t,n){this.view=e,this.stateField=t,this.applyCompletion=n,this.info=null,this.infoDestroy=null,this.placeInfoReq={read:()=>this.measureInfo(),write:l=>this.placeInfo(l),key:this},this.space=null,this.currentClass="";let s=e.state.field(t),{options:r,selected:o}=s.open,a=e.state.facet(le);this.optionContent=am(a),this.optionClass=a.optionClass,this.tooltipClass=a.tooltipClass,this.range=Qs(r.length,o,a.maxRenderedOptions),this.dom=document.createElement("div"),this.dom.className="cm-tooltip-autocomplete",this.updateTooltipClass(e.state),this.dom.addEventListener("mousedown",l=>{let{options:h}=e.state.field(t).open;for(let c=l.target,d;c&&c!=this.dom;c=c.parentNode)if(c.nodeName=="LI"&&(d=/-(\d+)$/.exec(c.id))&&+d[1]<h.length){this.applyCompletion(e,h[+d[1]]),l.preventDefault();return}if(l.target==this.list){let c=this.list.classList.contains("cm-completionListIncompleteTop")&&l.clientY<this.list.firstChild.getBoundingClientRect().top?this.range.from-1:this.list.classList.contains("cm-completionListIncompleteBottom")&&l.clientY>this.list.lastChild.getBoundingClientRect().bottom?this.range.to:null;c!=null&&(e.dispatch({effects:yo.of(c)}),l.preventDefault())}}),this.dom.addEventListener("focusout",l=>{let h=e.state.field(this.stateField,!1);h&&h.tooltip&&e.state.facet(le).closeOnBlur&&l.relatedTarget!=e.contentDOM&&e.dispatch({effects:Kn.of(null)})}),this.showOptions(r,s.id)}mount(){this.updateSel()}showOptions(e,t){this.list&&this.list.remove(),this.list=this.dom.appendChild(this.createListBox(e,t,this.range)),this.list.addEventListener("scroll",()=>{this.info&&this.view.requestMeasure(this.placeInfoReq)})}update(e){var t;let n=e.state.field(this.stateField),s=e.startState.field(this.stateField);if(this.updateTooltipClass(e.state),n!=s){let{options:r,selected:o,disabled:a}=n.open;(!s.open||s.open.options!=r)&&(this.range=Qs(r.length,o,e.state.facet(le).maxRenderedOptions),this.showOptions(r,n.id)),this.updateSel(),a!=((t=s.open)===null||t===void 0?void 0:t.disabled)&&this.dom.classList.toggle("cm-tooltip-autocomplete-disabled",!!a)}}updateTooltipClass(e){let t=this.tooltipClass(e);if(t!=this.currentClass){for(let n of this.currentClass.split(" "))n&&this.dom.classList.remove(n);for(let n of t.split(" "))n&&this.dom.classList.add(n);this.currentClass=t}}positioned(e){this.space=e,this.info&&this.view.requestMeasure(this.placeInfoReq)}updateSel(){let e=this.view.state.field(this.stateField),t=e.open;(t.selected>-1&&t.selected<this.range.from||t.selected>=this.range.to)&&(this.range=Qs(t.options.length,t.selected,this.view.state.facet(le).maxRenderedOptions),this.showOptions(t.options,e.id));let n=this.updateSelectedOption(t.selected);if(n){this.destroyInfo();let{completion:s}=t.options[t.selected],{info:r}=s;if(!r)return;let o=typeof r=="string"?document.createTextNode(r):r(s);if(!o)return;"then"in o?o.then(a=>{a&&this.view.state.field(this.stateField,!1)==e&&this.addInfoPane(a,s)}).catch(a=>Ce(this.view.state,a,"completion info")):(this.addInfoPane(o,s),n.setAttribute("aria-describedby",this.info.id))}}addInfoPane(e,t){this.destroyInfo();let n=this.info=document.createElement("div");if(n.className="cm-tooltip cm-completionInfo",n.id="cm-completionInfo-"+Math.floor(Math.random()*65535).toString(16),e.nodeType!=null)n.appendChild(e),this.infoDestroy=null;else{let{dom:s,destroy:r}=e;n.appendChild(s),this.infoDestroy=r||null}this.dom.appendChild(n),this.view.requestMeasure(this.placeInfoReq)}updateSelectedOption(e){let t=null;for(let n=this.list.firstChild,s=this.range.from;n;n=n.nextSibling,s++)n.nodeName!="LI"||!n.id?s--:s==e?n.hasAttribute("aria-selected")||(n.setAttribute("aria-selected","true"),t=n):n.hasAttribute("aria-selected")&&(n.removeAttribute("aria-selected"),n.removeAttribute("aria-describedby"));return t&&cm(this.list,t),t}measureInfo(){let e=this.dom.querySelector("[aria-selected]");if(!e||!this.info)return null;let t=this.dom.getBoundingClientRect(),n=this.info.getBoundingClientRect(),s=e.getBoundingClientRect(),r=this.space;if(!r){let o=this.dom.ownerDocument.documentElement;r={left:0,top:0,right:o.clientWidth,bottom:o.clientHeight}}return s.top>Math.min(r.bottom,t.bottom)-10||s.bottom<Math.max(r.top,t.top)+10?null:this.view.state.facet(le).positionInfo(this.view,t,s,n,r,this.dom)}placeInfo(e){this.info&&(e?(e.style&&(this.info.style.cssText=e.style),this.info.className="cm-tooltip cm-completionInfo "+(e.class||"")):this.info.style.cssText="top: -1e6px")}createListBox(e,t,n){const s=document.createElement("ul");s.id=t,s.setAttribute("role","listbox"),s.setAttribute("aria-expanded","true"),s.setAttribute("aria-label",this.view.state.phrase("Completions")),s.addEventListener("mousedown",o=>{o.target==s&&o.preventDefault()});let r=null;for(let o=n.from;o<n.to;o++){let{completion:a,match:l}=e[o],{section:h}=a;if(h){let u=typeof h=="string"?h:h.name;if(u!=r&&(o>n.from||n.from==0))if(r=u,typeof h!="string"&&h.header)s.appendChild(h.header(h));else{let f=s.appendChild(document.createElement("completion-section"));f.textContent=u}}const c=s.appendChild(document.createElement("li"));c.id=t+"-"+o,c.setAttribute("role","option");let d=this.optionClass(a);d&&(c.className=d);for(let u of this.optionContent){let f=u(a,this.view.state,this.view,l);f&&c.appendChild(f)}}return n.from&&s.classList.add("cm-completionListIncompleteTop"),n.to<e.length&&s.classList.add("cm-completionListIncompleteBottom"),s}destroyInfo(){this.info&&(this.infoDestroy&&this.infoDestroy(),this.info.remove(),this.info=null)}destroy(){this.destroyInfo()}}function hm(i,e){return t=>new lm(t,i,e)}function cm(i,e){let t=i.getBoundingClientRect(),n=e.getBoundingClientRect(),s=t.height/i.offsetHeight;n.top<t.top?i.scrollTop-=(t.top-n.top)/s:n.bottom>t.bottom&&(i.scrollTop+=(n.bottom-t.bottom)/s)}function $a(i){return(i.boost||0)*100+(i.apply?10:0)+(i.info?5:0)+(i.type?1:0)}function dm(i,e){let t=[],n=null,s=null,r=c=>{t.push(c);let{section:d}=c.completion;if(d){n||(n=[]);let u=typeof d=="string"?d:d.name;n.some(f=>f.name==u)||n.push(typeof d=="string"?{name:u}:d)}},o=e.facet(le);for(let c of i)if(c.hasResult()){let d=c.result.getMatch;if(c.result.filter===!1)for(let u of c.result.options)r(new za(u,c.source,d?d(u):[],1e9-t.length));else{let u=e.sliceDoc(c.from,c.to),f,p=o.filterStrict?new rm(u):new sm(u);for(let m of c.result.options)if(f=p.match(m.label)){let g=m.displayLabel?d?d(m,f.matched):[]:f.matched,y=f.score+(m.boost||0);if(r(new za(m,c.source,g,y)),typeof m.section=="object"&&m.section.rank==="dynamic"){let{name:x}=m.section;s||(s=Object.create(null)),s[x]=Math.max(y,s[x]||-1e9)}}}}if(n){let c=Object.create(null),d=0,u=(f,p)=>(f.rank==="dynamic"&&p.rank==="dynamic"?s[p.name]-s[f.name]:0)||(typeof f.rank=="number"?f.rank:1e9)-(typeof p.rank=="number"?p.rank:1e9)||(f.name<p.name?-1:1);for(let f of n.sort(u))d-=1e5,c[f.name]=d;for(let f of t){let{section:p}=f.completion;p&&(f.score+=c[typeof p=="string"?p:p.name])}}let a=[],l=null,h=o.compareCompletions;for(let c of t.sort((d,u)=>u.score-d.score||h(d.completion,u.completion))){let d=c.completion;!l||l.label!=d.label||l.detail!=d.detail||l.type!=null&&d.type!=null&&l.type!=d.type||l.apply!=d.apply||l.boost!=d.boost?a.push(c):$a(c.completion)>$a(l)&&(a[a.length-1]=c),l=c.completion}return a}class Xt{constructor(e,t,n,s,r,o){this.options=e,this.attrs=t,this.tooltip=n,this.timestamp=s,this.selected=r,this.disabled=o}setSelected(e,t){return e==this.selected||e>=this.options.length?this:new Xt(this.options,Ka(t,e),this.tooltip,this.timestamp,e,this.disabled)}static build(e,t,n,s,r,o){if(s&&!o&&e.some(h=>h.isPending))return s.setDisabled();let a=dm(e,t);if(!a.length)return s&&e.some(h=>h.isPending)?s.setDisabled():null;let l=t.facet(le).selectOnOpen?0:-1;if(s&&s.selected!=l&&s.selected!=-1){let h=s.options[s.selected].completion;for(let c=0;c<a.length;c++)if(a[c].completion==h){l=c;break}}return new Xt(a,Ka(n,l),{pos:e.reduce((h,c)=>c.hasResult()?Math.min(h,c.from):h,1e8),create:ym,above:r.aboveCursor},s?s.timestamp:Date.now(),l,!1)}map(e){return new Xt(this.options,this.attrs,{...this.tooltip,pos:e.mapPos(this.tooltip.pos)},this.timestamp,this.selected,this.disabled)}setDisabled(){return new Xt(this.options,this.attrs,this.tooltip,this.timestamp,this.selected,!0)}}class es{constructor(e,t,n){this.active=e,this.id=t,this.open=n}static start(){return new es(mm,"cm-ac-"+Math.floor(Math.random()*2e6).toString(36),null)}update(e){let{state:t}=e,n=t.facet(le),r=(n.override||t.languageDataAt("autocomplete",Ft(t)).map(im)).map(l=>(this.active.find(c=>c.source==l)||new He(l,this.active.some(c=>c.state!=0)?1:0)).update(e,n));r.length==this.active.length&&r.every((l,h)=>l==this.active[h])&&(r=this.active);let o=this.open,a=e.effects.some(l=>l.is(bo));o&&e.docChanged&&(o=o.map(e.changes)),e.selection||r.some(l=>l.hasResult()&&e.changes.touchesRange(l.from,l.to))||!um(r,this.active)||a?o=Xt.build(r,t,this.id,o,n,a):o&&o.disabled&&!r.some(l=>l.isPending)&&(o=null),!o&&r.every(l=>!l.isPending)&&r.some(l=>l.hasResult())&&(r=r.map(l=>l.hasResult()?new He(l.source,0):l));for(let l of e.effects)l.is(yo)&&(o=o&&o.setSelected(l.value,this.id));return r==this.active&&o==this.open?this:new es(r,this.id,o)}get tooltip(){return this.open?this.open.tooltip:null}get attrs(){return this.open?this.open.attrs:this.active.length?fm:pm}}function um(i,e){if(i==e)return!0;for(let t=0,n=0;;){for(;t<i.length&&!i[t].hasResult();)t++;for(;n<e.length&&!e[n].hasResult();)n++;let s=t==i.length,r=n==e.length;if(s||r)return s==r;if(i[t++].result!=e[n++].result)return!1}}const fm={"aria-autocomplete":"list"},pm={};function Ka(i,e){let t={"aria-autocomplete":"list","aria-haspopup":"listbox","aria-controls":i};return e>-1&&(t["aria-activedescendant"]=i+"-"+e),t}const mm=[];function Zh(i,e){if(i.isUserEvent("input.complete")){let n=i.annotation(Qh);if(n&&e.activateOnCompletion(n))return 12}let t=i.isUserEvent("input.type");return t&&e.activateOnTyping?5:t?1:i.isUserEvent("delete.backward")?2:i.selection?8:i.docChanged?16:0}class He{constructor(e,t,n=!1){this.source=e,this.state=t,this.explicit=n}hasResult(){return!1}get isPending(){return this.state==1}update(e,t){let n=Zh(e,t),s=this;(n&8||n&16&&this.touches(e))&&(s=new He(s.source,0)),n&4&&s.state==0&&(s=new He(this.source,1)),s=s.updateFor(e,n);for(let r of e.effects)if(r.is(Zi))s=new He(s.source,1,r.value);else if(r.is(Kn))s=new He(s.source,0);else if(r.is(bo))for(let o of r.value)o.source==s.source&&(s=o);return s}updateFor(e,t){return this.map(e.changes)}map(e){return this}touches(e){return e.changes.touchesRange(Ft(e.state))}}class nn extends He{constructor(e,t,n,s,r,o){super(e,3,t),this.limit=n,this.result=s,this.from=r,this.to=o}hasResult(){return!0}updateFor(e,t){var n;if(!(t&3))return this.map(e.changes);let s=this.result;s.map&&!e.changes.empty&&(s=s.map(s,e.changes));let r=e.changes.mapPos(this.from),o=e.changes.mapPos(this.to,1),a=Ft(e.state);if(a>o||!s||t&2&&(Ft(e.startState)==this.from||a<this.limit))return new He(this.source,t&4?1:0);let l=e.changes.mapPos(this.limit);return gm(s.validFor,e.state,r,o)?new nn(this.source,this.explicit,l,s,r,o):s.update&&(s=s.update(s,r,o,new Jh(e.state,a,!1)))?new nn(this.source,this.explicit,l,s,s.from,(n=s.to)!==null&&n!==void 0?n:Ft(e.state)):new He(this.source,1,this.explicit)}map(e){if(e.empty)return this;let t=this.result.map?this.result.map(this.result,e):this.result;return t?new nn(this.source,this.explicit,e.mapPos(this.limit),t,e.mapPos(this.from),e.mapPos(this.to,1)):new He(this.source,0)}touches(e){return e.changes.touchesRange(this.from,this.to)}}function gm(i,e,t,n){if(!i)return!1;let s=e.sliceDoc(t,n);return typeof i=="function"?i(s,t,n,e):Xh(i,!0).test(s)}const bo=N.define({map(i,e){return i.map(t=>t.map(e))}}),Ie=me.define({create(){return es.start()},update(i,e){return i.update(e)},provide:i=>[ms.from(i,e=>e.tooltip),R.contentAttributes.from(i,e=>e.attrs)]});function wo(i,e){const t=e.completion.apply||e.completion.label;let n=i.state.field(Ie).active.find(s=>s.source==e.source);return n instanceof nn?(typeof t=="string"?i.dispatch({...nm(i.state,t,n.from,n.to),annotations:Qh.of(e.completion)}):t(i,e.completion,n.from,n.to),!0):!1}const ym=hm(Ie,wo);function xi(i,e="option"){return t=>{let n=t.state.field(Ie,!1);if(!n||!n.open||n.open.disabled||Date.now()-n.open.timestamp<t.state.facet(le).interactionDelay)return!1;let s=1,r;e=="page"&&(r=_h(t,n.open.tooltip))&&(s=Math.max(2,Math.floor(r.dom.offsetHeight/r.dom.querySelector("li").offsetHeight)-1));let{length:o}=n.open.options,a=n.open.selected>-1?n.open.selected+s*(i?1:-1):i?0:o-1;return a<0?a=e=="page"?0:o-1:a>=o&&(a=e=="page"?o-1:0),t.dispatch({effects:yo.of(a)}),!0}}const bm=i=>{let e=i.state.field(Ie,!1);return i.state.readOnly||!e||!e.open||e.open.selected<0||e.open.disabled||Date.now()-e.open.timestamp<i.state.facet(le).interactionDelay?!1:wo(i,e.open.options[e.open.selected])},Zs=i=>i.state.field(Ie,!1)?(i.dispatch({effects:Zi.of(!0)}),!0):!1,wm=i=>{let e=i.state.field(Ie,!1);return!e||!e.active.some(t=>t.state!=0)?!1:(i.dispatch({effects:Kn.of(null)}),!0)};class vm{constructor(e,t){this.active=e,this.context=t,this.time=Date.now(),this.updates=[],this.done=void 0}}const km=50,xm=1e3,Sm=ce.fromClass(class{constructor(i){this.view=i,this.debounceUpdate=-1,this.running=[],this.debounceAccept=-1,this.pendingStart=!1,this.composing=0;for(let e of i.state.field(Ie).active)e.isPending&&this.startQuery(e)}update(i){let e=i.state.field(Ie),t=i.state.facet(le);if(!i.selectionSet&&!i.docChanged&&i.startState.field(Ie)==e)return;let n=i.transactions.some(r=>{let o=Zh(r,t);return o&8||(r.selection||r.docChanged)&&!(o&3)});for(let r=0;r<this.running.length;r++){let o=this.running[r];if(n||o.context.abortOnDocChange&&i.docChanged||o.updates.length+i.transactions.length>km&&Date.now()-o.time>xm){for(let a of o.context.abortListeners)try{a()}catch(l){Ce(this.view.state,l)}o.context.abortListeners=null,this.running.splice(r--,1)}else o.updates.push(...i.transactions)}this.debounceUpdate>-1&&clearTimeout(this.debounceUpdate),i.transactions.some(r=>r.effects.some(o=>o.is(Zi)))&&(this.pendingStart=!0);let s=this.pendingStart?50:t.activateOnTypingDelay;if(this.debounceUpdate=e.active.some(r=>r.isPending&&!this.running.some(o=>o.active.source==r.source))?setTimeout(()=>this.startUpdate(),s):-1,this.composing!=0)for(let r of i.transactions)r.isUserEvent("input.type")?this.composing=2:this.composing==2&&r.selection&&(this.composing=3)}startUpdate(){this.debounceUpdate=-1,this.pendingStart=!1;let{state:i}=this.view,e=i.field(Ie);for(let t of e.active)t.isPending&&!this.running.some(n=>n.active.source==t.source)&&this.startQuery(t);this.running.length&&e.open&&e.open.disabled&&(this.debounceAccept=setTimeout(()=>this.accept(),this.view.state.facet(le).updateSyncTime))}startQuery(i){let{state:e}=this.view,t=Ft(e),n=new Jh(e,t,i.explicit,this.view),s=new vm(i,n);this.running.push(s),Promise.resolve(i.source(n)).then(r=>{s.context.aborted||(s.done=r||null,this.scheduleAccept())},r=>{this.view.dispatch({effects:Kn.of(null)}),Ce(this.view.state,r)})}scheduleAccept(){this.running.every(i=>i.done!==void 0)?this.accept():this.debounceAccept<0&&(this.debounceAccept=setTimeout(()=>this.accept(),this.view.state.facet(le).updateSyncTime))}accept(){var i;this.debounceAccept>-1&&clearTimeout(this.debounceAccept),this.debounceAccept=-1;let e=[],t=this.view.state.facet(le),n=this.view.state.field(Ie);for(let s=0;s<this.running.length;s++){let r=this.running[s];if(r.done===void 0)continue;if(this.running.splice(s--,1),r.done){let a=Ft(r.updates.length?r.updates[0].startState:this.view.state),l=Math.min(a,r.done.from+(r.active.explicit?0:1)),h=new nn(r.active.source,r.active.explicit,l,r.done,r.done.from,(i=r.done.to)!==null&&i!==void 0?i:a);for(let c of r.updates)h=h.update(c,t);if(h.hasResult()){e.push(h);continue}}let o=n.active.find(a=>a.source==r.active.source);if(o&&o.isPending)if(r.done==null){let a=new He(r.active.source,0);for(let l of r.updates)a=a.update(l,t);a.isPending||e.push(a)}else this.startQuery(o)}(e.length||n.open&&n.open.disabled)&&this.view.dispatch({effects:bo.of(e)})}},{eventHandlers:{blur(i){let e=this.view.state.field(Ie,!1);if(e&&e.tooltip&&this.view.state.facet(le).closeOnBlur){let t=e.open&&_h(this.view,e.open.tooltip);(!t||!t.dom.contains(i.relatedTarget))&&setTimeout(()=>this.view.dispatch({effects:Kn.of(null)}),10)}},compositionstart(){this.composing=1},compositionend(){this.composing==3&&setTimeout(()=>this.view.dispatch({effects:Zi.of(!1)}),20),this.composing=0}}}),Cm=typeof navigator=="object"&&/Win/.test(navigator.platform),Am=pn.highest(R.domEventHandlers({keydown(i,e){let t=e.state.field(Ie,!1);if(!t||!t.open||t.open.disabled||t.open.selected<0||i.key.length>1||i.ctrlKey&&!(Cm&&i.altKey)||i.metaKey)return!1;let n=t.open.options[t.open.selected],s=t.active.find(o=>o.source==n.source),r=n.completion.commitCharacters||s.result.commitCharacters;return r&&r.indexOf(i.key)>-1&&wo(e,n),!1}})),Tm=R.baseTheme({".cm-tooltip.cm-tooltip-autocomplete":{"& > ul":{fontFamily:"monospace",whiteSpace:"nowrap",overflow:"hidden auto",maxWidth_fallback:"700px",maxWidth:"min(700px, 95vw)",minWidth:"250px",maxHeight:"10em",height:"100%",listStyle:"none",margin:0,padding:0,"& > li, & > completion-section":{padding:"1px 3px",lineHeight:1.2},"& > li":{overflowX:"hidden",textOverflow:"ellipsis",cursor:"pointer"},"& > completion-section":{display:"list-item",borderBottom:"1px solid silver",paddingLeft:"0.5em",opacity:.7}}},"&light .cm-tooltip-autocomplete ul li[aria-selected]":{background:"#17c",color:"white"},"&light .cm-tooltip-autocomplete-disabled ul li[aria-selected]":{background:"#777"},"&dark .cm-tooltip-autocomplete ul li[aria-selected]":{background:"#347",color:"white"},"&dark .cm-tooltip-autocomplete-disabled ul li[aria-selected]":{background:"#444"},".cm-completionListIncompleteTop:before, .cm-completionListIncompleteBottom:after":{content:'"···"',opacity:.5,display:"block",textAlign:"center",cursor:"pointer"},".cm-tooltip.cm-completionInfo":{position:"absolute",padding:"3px 9px",width:"max-content",maxWidth:"400px",boxSizing:"border-box",whiteSpace:"pre-line"},".cm-completionInfo.cm-completionInfo-left":{right:"100%"},".cm-completionInfo.cm-completionInfo-right":{left:"100%"},".cm-completionInfo.cm-completionInfo-left-narrow":{right:"30px"},".cm-completionInfo.cm-completionInfo-right-narrow":{left:"30px"},"&light .cm-snippetField":{backgroundColor:"#00000022"},"&dark .cm-snippetField":{backgroundColor:"#ffffff22"},".cm-snippetFieldPosition":{verticalAlign:"text-top",width:0,height:"1.15em",display:"inline-block",margin:"0 -0.7px -.7em",borderLeft:"1.4px dotted #888"},".cm-completionMatchedText":{textDecoration:"underline"},".cm-completionDetail":{marginLeft:"0.5em",fontStyle:"italic"},".cm-completionIcon":{fontSize:"90%",width:".8em",display:"inline-block",textAlign:"center",paddingRight:".6em",opacity:"0.6",boxSizing:"content-box"},".cm-completionIcon-function, .cm-completionIcon-method":{"&:after":{content:"'ƒ'"}},".cm-completionIcon-class":{"&:after":{content:"'○'"}},".cm-completionIcon-interface":{"&:after":{content:"'◌'"}},".cm-completionIcon-variable":{"&:after":{content:"'𝑥'"}},".cm-completionIcon-constant":{"&:after":{content:"'𝐶'"}},".cm-completionIcon-type":{"&:after":{content:"'𝑡'"}},".cm-completionIcon-enum":{"&:after":{content:"'∪'"}},".cm-completionIcon-property":{"&:after":{content:"'□'"}},".cm-completionIcon-keyword":{"&:after":{content:"'🔑︎'"}},".cm-completionIcon-namespace":{"&:after":{content:"'▢'"}},".cm-completionIcon-text":{"&:after":{content:"'abc'",fontSize:"50%",verticalAlign:"middle"}}}),Gn={brackets:["(","[","{","'",'"'],before:")]}:;>",stringPrefixes:[]},Wt=N.define({map(i,e){let t=e.mapPos(i,-1,Se.TrackAfter);return t??void 0}}),vo=new class extends Mt{};vo.startSide=1;vo.endSide=-1;const ec=me.define({create(){return H.empty},update(i,e){if(i=i.map(e.changes),e.selection){let t=e.state.doc.lineAt(e.selection.main.head);i=i.update({filter:n=>n>=t.from&&n<=t.to})}for(let t of e.effects)t.is(Wt)&&(i=i.update({add:[vo.range(t.value,t.value+1)]}));return i}});function Mm(){return[Im,ec]}const er="()[]{}<>«»»«［］｛｝";function tc(i){for(let e=0;e<er.length;e+=2)if(er.charCodeAt(e)==i)return er.charAt(e+1);return Gr(i<128?i:i+1)}function nc(i,e){return i.languageDataAt("closeBrackets",e)[0]||Gn}const Om=typeof navigator=="object"&&/Android\b/.test(navigator.userAgent),Im=R.inputHandler.of((i,e,t,n)=>{if((Om?i.composing:i.compositionStarted)||i.state.readOnly)return!1;let s=i.state.selection.main;if(n.length>2||n.length==2&&rt(_e(n,0))==1||e!=s.from||t!=s.to)return!1;let r=Em(i.state,n);return r?(i.dispatch(r),!0):!1}),Dm=({state:i,dispatch:e})=>{if(i.readOnly)return!1;let n=nc(i,i.selection.main.head).brackets||Gn.brackets,s=null,r=i.changeByRange(o=>{if(o.empty){let a=Lm(i.doc,o.head);for(let l of n)if(l==a&&vs(i.doc,o.head)==tc(_e(l,0)))return{changes:{from:o.head-l.length,to:o.head+l.length},range:b.cursor(o.head-l.length)}}return{range:s=o}});return s||e(i.update(r,{scrollIntoView:!0,userEvent:"delete.backward"})),!s},Rm=[{key:"Backspace",run:Dm}];function Em(i,e){let t=nc(i,i.selection.main.head),n=t.brackets||Gn.brackets;for(let s of n){let r=tc(_e(s,0));if(e==s)return r==s?Pm(i,s,n.indexOf(s+s+s)>-1,t):Bm(i,s,r,t.before||Gn.before);if(e==r&&ic(i,i.selection.main.from))return _m(i,s,r)}return null}function ic(i,e){let t=!1;return i.field(ec).between(0,i.doc.length,n=>{n==e&&(t=!0)}),t}function vs(i,e){let t=i.sliceString(e,e+2);return t.slice(0,rt(_e(t,0)))}function Lm(i,e){let t=i.sliceString(e-2,e);return rt(_e(t,0))==t.length?t:t.slice(1)}function Bm(i,e,t,n){let s=null,r=i.changeByRange(o=>{if(!o.empty)return{changes:[{insert:e,from:o.from},{insert:t,from:o.to}],effects:Wt.of(o.to+e.length),range:b.range(o.anchor+e.length,o.head+e.length)};let a=vs(i.doc,o.head);return!a||/\s/.test(a)||n.indexOf(a)>-1?{changes:{insert:e+t,from:o.head},effects:Wt.of(o.head+e.length),range:b.cursor(o.head+e.length)}:{range:s=o}});return s?null:i.update(r,{scrollIntoView:!0,userEvent:"input.type"})}function _m(i,e,t){let n=null,s=i.changeByRange(r=>r.empty&&vs(i.doc,r.head)==t?{changes:{from:r.head,to:r.head+t.length,insert:t},range:b.cursor(r.head+t.length)}:n={range:r});return n?null:i.update(s,{scrollIntoView:!0,userEvent:"input.type"})}function Pm(i,e,t,n){let s=n.stringPrefixes||Gn.stringPrefixes,r=null,o=i.changeByRange(a=>{if(!a.empty)return{changes:[{insert:e,from:a.from},{insert:e,from:a.to}],effects:Wt.of(a.to+e.length),range:b.range(a.anchor+e.length,a.head+e.length)};let l=a.head,h=vs(i.doc,l),c;if(h==e){if(Ga(i,l))return{changes:{insert:e+e,from:l},effects:Wt.of(l+e.length),range:b.cursor(l+e.length)};if(ic(i,l)){let u=t&&i.sliceDoc(l,l+e.length*3)==e+e+e?e+e+e:e;return{changes:{from:l,to:l+u.length,insert:u},range:b.cursor(l+u.length)}}}else{if(t&&i.sliceDoc(l-2*e.length,l)==e+e&&(c=Ya(i,l-2*e.length,s))>-1&&Ga(i,c))return{changes:{insert:e+e+e+e,from:l},effects:Wt.of(l+e.length),range:b.cursor(l+e.length)};if(i.charCategorizer(l)(h)!=Z.Word&&Ya(i,l,s)>-1&&!Nm(i,l,e,s))return{changes:{insert:e+e,from:l},effects:Wt.of(l+e.length),range:b.cursor(l+e.length)}}return{range:r=a}});return r?null:i.update(o,{scrollIntoView:!0,userEvent:"input.type"})}function Ga(i,e){let t=Je(i).resolveInner(e+1);return t.parent&&t.from==e}function Nm(i,e,t,n){let s=Je(i).resolveInner(e,-1),r=n.reduce((o,a)=>Math.max(o,a.length),0);for(let o=0;o<5;o++){let a=i.sliceDoc(s.from,Math.min(s.to,s.from+t.length+r)),l=a.indexOf(t);if(!l||l>-1&&n.indexOf(a.slice(0,l))>-1){let c=s.firstChild;for(;c&&c.from==s.from&&c.to-c.from>t.length+l;){if(i.sliceDoc(c.to-t.length,c.to)==t)return!1;c=c.firstChild}return!0}let h=s.to==e&&s.parent;if(!h)break;s=h}return!1}function Ya(i,e,t){let n=i.charCategorizer(e);if(n(i.sliceDoc(e-1,e))!=Z.Word)return e;for(let s of t){let r=e-s.length;if(i.sliceDoc(r,e)==s&&n(i.sliceDoc(r-1,r))!=Z.Word)return r}return-1}function Wm(i={}){return[Am,Ie,le.of(i),Sm,qm,Tm]}const sc=[{key:"Ctrl-Space",run:Zs},{mac:"Alt-`",run:Zs},{mac:"Alt-i",run:Zs},{key:"Escape",run:wm},{key:"ArrowDown",run:xi(!0)},{key:"ArrowUp",run:xi(!1)},{key:"PageDown",run:xi(!0,"page")},{key:"PageUp",run:xi(!1,"page")},{key:"Enter",run:bm}],qm=pn.highest(co.computeN([le],i=>i.facet(le).defaultKeymap?[sc]:[])),Hm=i=>{let{state:e}=i,t=e.doc.lineAt(e.selection.main.from),n=xo(i.state,t.from);return n.line?Fm(i):n.block?zm(i):!1};function ko(i,e){return({state:t,dispatch:n})=>{if(t.readOnly)return!1;let s=i(e,t);return s?(n(t.update(s)),!0):!1}}const Fm=ko($m,0),Vm=ko(rc,0),zm=ko((i,e)=>rc(i,e,Um(e)),0);function xo(i,e){let t=i.languageDataAt("commentTokens",e,1);return t.length?t[0]:{}}const Tn=50;function jm(i,{open:e,close:t},n,s){let r=i.sliceDoc(n-Tn,n),o=i.sliceDoc(s,s+Tn),a=/\s*$/.exec(r)[0].length,l=/^\s*/.exec(o)[0].length,h=r.length-a;if(r.slice(h-e.length,h)==e&&o.slice(l,l+t.length)==t)return{open:{pos:n-a,margin:a&&1},close:{pos:s+l,margin:l&&1}};let c,d;s-n<=2*Tn?c=d=i.sliceDoc(n,s):(c=i.sliceDoc(n,n+Tn),d=i.sliceDoc(s-Tn,s));let u=/^\s*/.exec(c)[0].length,f=/\s*$/.exec(d)[0].length,p=d.length-f-t.length;return c.slice(u,u+e.length)==e&&d.slice(p,p+t.length)==t?{open:{pos:n+u+e.length,margin:/\s/.test(c.charAt(u+e.length))?1:0},close:{pos:s-f-t.length,margin:/\s/.test(d.charAt(p-1))?1:0}}:null}function Um(i){let e=[];for(let t of i.selection.ranges){let n=i.doc.lineAt(t.from),s=t.to<=n.to?n:i.doc.lineAt(t.to);s.from>n.from&&s.from==t.to&&(s=t.to==n.to+1?n:i.doc.lineAt(t.to-1));let r=e.length-1;r>=0&&e[r].to>n.from?e[r].to=s.to:e.push({from:n.from+/^\s*/.exec(n.text)[0].length,to:s.to})}return e}function rc(i,e,t=e.selection.ranges){let n=t.map(r=>xo(e,r.from).block);if(!n.every(r=>r))return null;let s=t.map((r,o)=>jm(e,n[o],r.from,r.to));if(i!=2&&!s.every(r=>r))return{changes:e.changes(t.map((r,o)=>s[o]?[]:[{from:r.from,insert:n[o].open+" "},{from:r.to,insert:" "+n[o].close}]))};if(i!=1&&s.some(r=>r)){let r=[];for(let o=0,a;o<s.length;o++)if(a=s[o]){let l=n[o],{open:h,close:c}=a;r.push({from:h.pos-l.open.length,to:h.pos+h.margin},{from:c.pos-c.margin,to:c.pos+l.close.length})}return{changes:r}}return null}function $m(i,e,t=e.selection.ranges){let n=[],s=-1;e:for(let{from:r,to:o}of t){let a=n.length,l=1e9,h;for(let c=r;c<=o;){let d=e.doc.lineAt(c);if(h==null&&(h=xo(e,d.from).line,!h))continue e;if(d.from>s&&(r==o||o>d.from)){s=d.from;let u=/^\s*/.exec(d.text)[0].length,f=u==d.length,p=d.text.slice(u,u+h.length)==h?u:-1;u<d.text.length&&u<l&&(l=u),n.push({line:d,comment:p,token:h,indent:u,empty:f,single:!1})}c=d.to+1}if(l<1e9)for(let c=a;c<n.length;c++)n[c].indent<n[c].line.text.length&&(n[c].indent=l);n.length==a+1&&(n[a].single=!0)}if(i!=2&&n.some(r=>r.comment<0&&(!r.empty||r.single))){let r=[];for(let{line:a,token:l,indent:h,empty:c,single:d}of n)(d||!c)&&r.push({from:a.from+h,insert:l+" "});let o=e.changes(r);return{changes:o,selection:e.selection.map(o,1)}}else if(i!=1&&n.some(r=>r.comment>=0)){let r=[];for(let{line:o,comment:a,token:l}of n)if(a>=0){let h=o.from+a,c=h+l.length;o.text[c-o.from]==" "&&c++,r.push({from:h,to:c})}return{changes:r}}return null}const Vr=yt.define(),Km=yt.define(),Gm=D.define(),oc=D.define({combine(i){return bt(i,{minDepth:100,newGroupDelay:500,joinToEvent:(e,t)=>t},{minDepth:Math.max,newGroupDelay:Math.min,joinToEvent:(e,t)=>(n,s)=>e(n,s)||t(n,s)})}}),ac=me.define({create(){return ht.empty},update(i,e){let t=e.state.facet(oc),n=e.annotation(Vr);if(n){let l=Re.fromTransaction(e,n.selection),h=n.side,c=h==0?i.undone:i.done;return l?c=ts(c,c.length,t.minDepth,l):c=cc(c,e.startState.selection),new ht(h==0?n.rest:c,h==0?c:n.rest)}let s=e.annotation(Km);if((s=="full"||s=="before")&&(i=i.isolate()),e.annotation(ae.addToHistory)===!1)return e.changes.empty?i:i.addMapping(e.changes.desc);let r=Re.fromTransaction(e),o=e.annotation(ae.time),a=e.annotation(ae.userEvent);return r?i=i.addChanges(r,o,a,t,e):e.selection&&(i=i.addSelection(e.startState.selection,o,a,t.newGroupDelay)),(s=="full"||s=="after")&&(i=i.isolate()),i},toJSON(i){return{done:i.done.map(e=>e.toJSON()),undone:i.undone.map(e=>e.toJSON())}},fromJSON(i){return new ht(i.done.map(Re.fromJSON),i.undone.map(Re.fromJSON))}});function Ym(i={}){return[ac,oc.of(i),R.domEventHandlers({beforeinput(e,t){let n=e.inputType=="historyUndo"?lc:e.inputType=="historyRedo"?zr:null;return n?(e.preventDefault(),n(t)):!1}})]}function ks(i,e){return function({state:t,dispatch:n}){if(!e&&t.readOnly)return!1;let s=t.field(ac,!1);if(!s)return!1;let r=s.pop(i,t,e);return r?(n(r),!0):!1}}const lc=ks(0,!1),zr=ks(1,!1),Jm=ks(0,!0),Xm=ks(1,!0);class Re{constructor(e,t,n,s,r){this.changes=e,this.effects=t,this.mapped=n,this.startSelection=s,this.selectionsAfter=r}setSelAfter(e){return new Re(this.changes,this.effects,this.mapped,this.startSelection,e)}toJSON(){var e,t,n;return{changes:(e=this.changes)===null||e===void 0?void 0:e.toJSON(),mapped:(t=this.mapped)===null||t===void 0?void 0:t.toJSON(),startSelection:(n=this.startSelection)===null||n===void 0?void 0:n.toJSON(),selectionsAfter:this.selectionsAfter.map(s=>s.toJSON())}}static fromJSON(e){return new Re(e.changes&&oe.fromJSON(e.changes),[],e.mapped&&dt.fromJSON(e.mapped),e.startSelection&&b.fromJSON(e.startSelection),e.selectionsAfter.map(b.fromJSON))}static fromTransaction(e,t){let n=Fe;for(let s of e.startState.facet(Gm)){let r=s(e);r.length&&(n=n.concat(r))}return!n.length&&e.changes.empty?null:new Re(e.changes.invert(e.startState.doc),n,void 0,t||e.startState.selection,Fe)}static selection(e){return new Re(void 0,Fe,void 0,void 0,e)}}function ts(i,e,t,n){let s=e+1>t+20?e-t-1:0,r=i.slice(s,e);return r.push(n),r}function Qm(i,e){let t=[],n=!1;return i.iterChangedRanges((s,r)=>t.push(s,r)),e.iterChangedRanges((s,r,o,a)=>{for(let l=0;l<t.length;){let h=t[l++],c=t[l++];a>=h&&o<=c&&(n=!0)}}),n}function Zm(i,e){return i.ranges.length==e.ranges.length&&i.ranges.filter((t,n)=>t.empty!=e.ranges[n].empty).length===0}function hc(i,e){return i.length?e.length?i.concat(e):i:e}const Fe=[],eg=200;function cc(i,e){if(i.length){let t=i[i.length-1],n=t.selectionsAfter.slice(Math.max(0,t.selectionsAfter.length-eg));return n.length&&n[n.length-1].eq(e)?i:(n.push(e),ts(i,i.length-1,1e9,t.setSelAfter(n)))}else return[Re.selection([e])]}function tg(i){let e=i[i.length-1],t=i.slice();return t[i.length-1]=e.setSelAfter(e.selectionsAfter.slice(0,e.selectionsAfter.length-1)),t}function tr(i,e){if(!i.length)return i;let t=i.length,n=Fe;for(;t;){let s=ng(i[t-1],e,n);if(s.changes&&!s.changes.empty||s.effects.length){let r=i.slice(0,t);return r[t-1]=s,r}else e=s.mapped,t--,n=s.selectionsAfter}return n.length?[Re.selection(n)]:Fe}function ng(i,e,t){let n=hc(i.selectionsAfter.length?i.selectionsAfter.map(a=>a.map(e)):Fe,t);if(!i.changes)return Re.selection(n);let s=i.changes.map(e),r=e.mapDesc(i.changes,!0),o=i.mapped?i.mapped.composeDesc(r):r;return new Re(s,N.mapEffects(i.effects,e),o,i.startSelection.map(r),n)}const ig=/^(input\.type|delete)($|\.)/;class ht{constructor(e,t,n=0,s=void 0){this.done=e,this.undone=t,this.prevTime=n,this.prevUserEvent=s}isolate(){return this.prevTime?new ht(this.done,this.undone):this}addChanges(e,t,n,s,r){let o=this.done,a=o[o.length-1];return a&&a.changes&&!a.changes.empty&&e.changes&&(!n||ig.test(n))&&(!a.selectionsAfter.length&&t-this.prevTime<s.newGroupDelay&&s.joinToEvent(r,Qm(a.changes,e.changes))||n=="input.type.compose")?o=ts(o,o.length-1,s.minDepth,new Re(e.changes.compose(a.changes),hc(N.mapEffects(e.effects,a.changes),a.effects),a.mapped,a.startSelection,Fe)):o=ts(o,o.length,s.minDepth,e),new ht(o,Fe,t,n)}addSelection(e,t,n,s){let r=this.done.length?this.done[this.done.length-1].selectionsAfter:Fe;return r.length>0&&t-this.prevTime<s&&n==this.prevUserEvent&&n&&/^select($|\.)/.test(n)&&Zm(r[r.length-1],e)?this:new ht(cc(this.done,e),this.undone,t,n)}addMapping(e){return new ht(tr(this.done,e),tr(this.undone,e),this.prevTime,this.prevUserEvent)}pop(e,t,n){let s=e==0?this.done:this.undone;if(s.length==0)return null;let r=s[s.length-1],o=r.selectionsAfter[0]||(r.startSelection?r.startSelection.map(r.changes.invertedDesc,1):t.selection);if(n&&r.selectionsAfter.length)return t.update({selection:r.selectionsAfter[r.selectionsAfter.length-1],annotations:Vr.of({side:e,rest:tg(s),selection:o}),userEvent:e==0?"select.undo":"select.redo",scrollIntoView:!0});if(r.changes){let a=s.length==1?Fe:s.slice(0,s.length-1);return r.mapped&&(a=tr(a,r.mapped)),t.update({changes:r.changes,selection:r.startSelection,effects:r.effects,annotations:Vr.of({side:e,rest:a,selection:o}),filter:!1,userEvent:e==0?"undo":"redo",scrollIntoView:!0})}else return null}}ht.empty=new ht(Fe,Fe);const sg=[{key:"Mod-z",run:lc,preventDefault:!0},{key:"Mod-y",mac:"Mod-Shift-z",run:zr,preventDefault:!0},{linux:"Ctrl-Shift-z",run:zr,preventDefault:!0},{key:"Mod-u",run:Jm,preventDefault:!0},{key:"Alt-u",mac:"Mod-Shift-u",run:Xm,preventDefault:!0}];function mn(i,e){return b.create(i.ranges.map(e),i.mainIndex)}function Xe(i,e){return i.update({selection:e,scrollIntoView:!0,userEvent:"select"})}function Qe({state:i,dispatch:e},t){let n=mn(i.selection,t);return n.eq(i.selection,!0)?!1:(e(Xe(i,n)),!0)}function xs(i,e){return b.cursor(e?i.to:i.from)}function dc(i,e){return Qe(i,t=>t.empty?i.moveByChar(t,e):xs(t,e))}function ve(i){return i.textDirectionAt(i.state.selection.main.head)==J.LTR}const uc=i=>dc(i,!ve(i)),fc=i=>dc(i,ve(i));function pc(i,e){return Qe(i,t=>t.empty?i.moveByGroup(t,e):xs(t,e))}const rg=i=>pc(i,!ve(i)),og=i=>pc(i,ve(i));function ag(i,e,t){if(e.type.prop(t))return!0;let n=e.to-e.from;return n&&(n>2||/[^\s,.;:]/.test(i.sliceDoc(e.from,e.to)))||e.firstChild}function Ss(i,e,t){let n=Je(i).resolveInner(e.head),s=t?U.closedBy:U.openedBy;for(let l=e.head;;){let h=t?n.childAfter(l):n.childBefore(l);if(!h)break;ag(i,h,s)?n=h:l=t?h.to:h.from}let r=n.type.prop(s),o,a;return r&&(o=t?lt(i,n.from,1):lt(i,n.to,-1))&&o.matched?a=t?o.end.to:o.end.from:a=t?n.to:n.from,b.cursor(a,t?-1:1)}const lg=i=>Qe(i,e=>Ss(i.state,e,!ve(i))),hg=i=>Qe(i,e=>Ss(i.state,e,ve(i)));function mc(i,e){return Qe(i,t=>{if(!t.empty)return xs(t,e);let n=i.moveVertically(t,e);return n.head!=t.head?n:i.moveToLineBoundary(t,e)})}const gc=i=>mc(i,!1),yc=i=>mc(i,!0);function bc(i){let e=i.scrollDOM.clientHeight<i.scrollDOM.scrollHeight-2,t=0,n=0,s;if(e){for(let r of i.state.facet(R.scrollMargins)){let o=r(i);o!=null&&o.top&&(t=Math.max(o==null?void 0:o.top,t)),o!=null&&o.bottom&&(n=Math.max(o==null?void 0:o.bottom,n))}s=i.scrollDOM.clientHeight-t-n}else s=(i.dom.ownerDocument.defaultView||window).innerHeight;return{marginTop:t,marginBottom:n,selfScroll:e,height:Math.max(i.defaultLineHeight,s-5)}}function wc(i,e){let t=bc(i),{state:n}=i,s=mn(n.selection,o=>o.empty?i.moveVertically(o,e,t.height):xs(o,e));if(s.eq(n.selection))return!1;let r;if(t.selfScroll){let o=i.coordsAtPos(n.selection.main.head),a=i.scrollDOM.getBoundingClientRect(),l=a.top+t.marginTop,h=a.bottom-t.marginBottom;o&&o.top>l&&o.bottom<h&&(r=R.scrollIntoView(s.main.head,{y:"start",yMargin:o.top-l}))}return i.dispatch(Xe(n,s),{effects:r}),!0}const Ja=i=>wc(i,!1),jr=i=>wc(i,!0);function Et(i,e,t){let n=i.lineBlockAt(e.head),s=i.moveToLineBoundary(e,t);if(s.head==e.head&&s.head!=(t?n.to:n.from)&&(s=i.moveToLineBoundary(e,t,!1)),!t&&s.head==n.from&&n.length){let r=/^\s*/.exec(i.state.sliceDoc(n.from,Math.min(n.from+100,n.to)))[0].length;r&&e.head!=n.from+r&&(s=b.cursor(n.from+r))}return s}const cg=i=>Qe(i,e=>Et(i,e,!0)),dg=i=>Qe(i,e=>Et(i,e,!1)),ug=i=>Qe(i,e=>Et(i,e,!ve(i))),fg=i=>Qe(i,e=>Et(i,e,ve(i))),pg=i=>Qe(i,e=>i.moveToLineBoundary(e,!1,!1)),mg=i=>Qe(i,e=>i.moveToLineBoundary(e,!0,!1));function gg(i,e,t){let n=!1,s=mn(i.selection,r=>{let o=lt(i,r.head,-1)||lt(i,r.head,1)||r.head>0&&lt(i,r.head-1,1)||r.head<i.doc.length&&lt(i,r.head+1,-1);if(!o||!o.end)return r;n=!0;let a=o.start.from==r.head?o.end.to:o.end.from;return b.cursor(a)});return n?(e(Xe(i,s)),!0):!1}const yg=({state:i,dispatch:e})=>gg(i,e);function je(i,e,t){let n=mn(i.state.selection,s=>{s.undirectional&&s.head>=s.anchor!=e&&(s=b.range(s.head,s.anchor));let r=t(s);return b.range(s.anchor,r.head,r.goalColumn,r.bidiLevel||void 0,r.assoc)});return n.eq(i.state.selection)?!1:(i.dispatch(Xe(i.state,n)),!0)}function vc(i,e){return je(i,e,t=>i.moveByChar(t,e))}const kc=i=>vc(i,!ve(i)),xc=i=>vc(i,ve(i));function Sc(i,e){return je(i,e,t=>i.moveByGroup(t,e))}const bg=i=>Sc(i,!ve(i)),wg=i=>Sc(i,ve(i)),vg=i=>{let e=!ve(i);return je(i,e,t=>Ss(i.state,t,e))},kg=i=>{let e=ve(i);return je(i,e,t=>Ss(i.state,t,e))};function Cc(i,e){return je(i,e,t=>i.moveVertically(t,e))}const Ac=i=>Cc(i,!1),Tc=i=>Cc(i,!0);function Mc(i,e){return je(i,e,t=>i.moveVertically(t,e,bc(i).height))}const Xa=i=>Mc(i,!1),Qa=i=>Mc(i,!0),xg=i=>je(i,!0,e=>Et(i,e,!0)),Sg=i=>je(i,!1,e=>Et(i,e,!1)),Cg=i=>{let e=!ve(i);return je(i,e,t=>Et(i,t,e))},Ag=i=>{let e=ve(i);return je(i,e,t=>Et(i,t,e))},Tg=i=>je(i,!1,e=>b.cursor(i.lineBlockAt(e.head).from)),Mg=i=>je(i,!0,e=>b.cursor(i.lineBlockAt(e.head).to)),Za=({state:i,dispatch:e})=>(e(Xe(i,{anchor:0})),!0),el=({state:i,dispatch:e})=>(e(Xe(i,{anchor:i.doc.length})),!0),tl=({state:i,dispatch:e})=>(e(Xe(i,{anchor:i.selection.main.anchor,head:0})),!0),nl=({state:i,dispatch:e})=>(e(Xe(i,{anchor:i.selection.main.anchor,head:i.doc.length})),!0),Og=({state:i,dispatch:e})=>(e(i.update({selection:{anchor:0,head:i.doc.length},userEvent:"select"})),!0),Ig=({state:i,dispatch:e})=>{let t=Cs(i).map(({from:n,to:s})=>b.undirectionalRange(n,Math.min(s+1,i.doc.length)));return e(i.update({selection:b.create(t),userEvent:"select"})),!0},Dg=({state:i,dispatch:e})=>{let t=mn(i.selection,n=>{let s=Je(i),r=s.resolveStack(n.from,1);if(n.empty){let o=s.resolveStack(n.from,-1);o.node.from>=r.node.from&&o.node.to<=r.node.to&&(r=o)}for(let o=r;o;o=o.next){let{node:a}=o;if((a.from<n.from&&a.to>=n.to||a.to>n.to&&a.from<=n.from)&&o.next)return b.undirectionalRange(a.from,a.to)}return n});return t.eq(i.selection)?!1:(e(Xe(i,t)),!0)};function Oc(i,e){let{state:t}=i,n=t.selection,s=t.selection.ranges.slice();for(let r of t.selection.ranges){let o=t.doc.lineAt(r.head);if(e?o.to<i.state.doc.length:o.from>0)for(let a=r;;){let l=i.moveVertically(a,e);if(l.head<o.from||l.head>o.to){s.some(h=>h.head==l.head)||s.push(l);break}else{if(l.head==a.head)break;a=l}}}return s.length==n.ranges.length?!1:(i.dispatch(Xe(t,b.create(s,s.length-1))),!0)}const Rg=i=>Oc(i,!1),Eg=i=>Oc(i,!0),Lg=({state:i,dispatch:e})=>{let t=i.selection,n=null;return t.ranges.length>1?n=b.create([t.main]):t.main.empty||(n=b.create([b.cursor(t.main.head)])),n?(e(Xe(i,n)),!0):!1};function ti(i,e){if(i.state.readOnly)return!1;let t="delete.selection",{state:n}=i,s=n.changeByRange(r=>{let{from:o,to:a}=r;if(o==a){let l=e(r);l<o?(t="delete.backward",l=Si(i,l,!1)):l>o&&(t="delete.forward",l=Si(i,l,!0)),o=Math.min(o,l),a=Math.max(a,l)}else o=Si(i,o,!1),a=Si(i,a,!0);return o==a?{range:r}:{changes:{from:o,to:a},range:b.cursor(o,o<r.head?-1:1)}});return s.changes.empty?!1:(i.dispatch(n.update(s,{scrollIntoView:!0,userEvent:t,effects:t=="delete.selection"?R.announce.of(n.phrase("Selection deleted")):void 0})),!0)}function Si(i,e,t){if(i instanceof R)for(let n of i.state.facet(R.atomicRanges).map(s=>s(i)))n.between(e,e,(s,r)=>{s<e&&r>e&&(e=t?r:s)});return e}const Ic=(i,e,t)=>ti(i,n=>{let s=n.from,{state:r}=i,o=r.doc.lineAt(s),a,l;if(t&&!e&&s>o.from&&s<o.from+200&&!/[^ \t]/.test(a=o.text.slice(0,s-o.from))){if(a[a.length-1]=="	")return s-1;let h=hs(a,r.tabSize),c=h%Xi(r)||Xi(r);for(let d=0;d<c&&a[a.length-1-d]==" ";d++)s--;l=s}else l=he(o.text,s-o.from,e,e)+o.from,l==s&&o.number!=(e?r.doc.lines:1)?l+=e?1:-1:!e&&/[\ufe00-\ufe0f]/.test(o.text.slice(l-o.from,s-o.from))&&(l=he(o.text,l-o.from,!1,!1)+o.from);return l}),Ur=i=>Ic(i,!1,!0),Dc=i=>Ic(i,!0,!1),Rc=(i,e)=>ti(i,t=>{let n=t.head,{state:s}=i,r=s.doc.lineAt(n),o=s.charCategorizer(n);for(let a=null;;){if(n==(e?r.to:r.from)){n==t.head&&r.number!=(e?s.doc.lines:1)&&(n+=e?1:-1);break}let l=he(r.text,n-r.from,e)+r.from,h=r.text.slice(Math.min(n,l)-r.from,Math.max(n,l)-r.from),c=o(h);if(a!=null&&c!=a)break;(h!=" "||n!=t.head)&&(a=c),n=l}return n}),Ec=i=>Rc(i,!1),Bg=i=>Rc(i,!0),_g=i=>ti(i,e=>{let t=i.lineBlockAt(e.head).to;return e.head<t?t:Math.min(i.state.doc.length,e.head+1)}),Pg=i=>ti(i,e=>{let t=i.moveToLineBoundary(e,!1).head;return e.head>t?t:Math.max(0,e.head-1)}),Ng=i=>ti(i,e=>{let t=i.moveToLineBoundary(e,!0).head;return e.head<t?t:Math.min(i.state.doc.length,e.head+1)}),Wg=({state:i,dispatch:e})=>{if(i.readOnly)return!1;let t=i.changeByRange(n=>({changes:{from:n.from,to:n.to,insert:j.of(["",""])},range:b.cursor(n.from)}));return e(i.update(t,{scrollIntoView:!0,userEvent:"input"})),!0},qg=({state:i,dispatch:e})=>{if(i.readOnly)return!1;let t=i.changeByRange(n=>{if(!n.empty||n.from==0||n.from==i.doc.length)return{range:n};let s=n.from,r=i.doc.lineAt(s),o=s==r.from?s-1:he(r.text,s-r.from,!1)+r.from,a=s==r.to?s+1:he(r.text,s-r.from,!0)+r.from;return{changes:{from:o,to:a,insert:i.doc.slice(s,a).append(i.doc.slice(o,s))},range:b.cursor(a)}});return t.changes.empty?!1:(e(i.update(t,{scrollIntoView:!0,userEvent:"move.character"})),!0)};function Cs(i){let e=[],t=-1;for(let n of i.selection.ranges){let s=i.doc.lineAt(n.from),r=i.doc.lineAt(n.to);if(!n.empty&&n.to==r.from&&(r=i.doc.lineAt(n.to-1)),t>=s.number){let o=e[e.length-1];o.to=r.to,o.ranges.push(n)}else e.push({from:s.from,to:r.to,ranges:[n]});t=r.number+1}return e}function Lc(i,e,t){if(i.readOnly)return!1;let n=[],s=[];for(let r of Cs(i)){if(t?r.to==i.doc.length:r.from==0)continue;let o=i.doc.lineAt(t?r.to+1:r.from-1),a=o.length+1;if(t){n.push({from:r.to,to:o.to},{from:r.from,insert:o.text+i.lineBreak});for(let l of r.ranges)s.push(b.range(Math.min(i.doc.length,l.anchor+a),Math.min(i.doc.length,l.head+a)))}else{n.push({from:o.from,to:r.from},{from:r.to,insert:i.lineBreak+o.text});for(let l of r.ranges)s.push(b.range(l.anchor-a,l.head-a))}}return n.length?(e(i.update({changes:n,scrollIntoView:!0,selection:b.create(s,i.selection.mainIndex),userEvent:"move.line"})),!0):!1}const Hg=({state:i,dispatch:e})=>Lc(i,e,!1),Fg=({state:i,dispatch:e})=>Lc(i,e,!0);function Bc(i,e,t){if(i.readOnly)return!1;let n=[];for(let r of Cs(i))t?n.push({from:r.from,insert:i.doc.slice(r.from,r.to)+i.lineBreak}):n.push({from:r.to,insert:i.lineBreak+i.doc.slice(r.from,r.to)});let s=i.changes(n);return e(i.update({changes:s,selection:i.selection.map(s,t?1:-1),scrollIntoView:!0,userEvent:"input.copyline"})),!0}const Vg=({state:i,dispatch:e})=>Bc(i,e,!1),zg=({state:i,dispatch:e})=>Bc(i,e,!0),jg=i=>{if(i.state.readOnly)return!1;let{state:e}=i,t=e.changes(Cs(e).map(({from:s,to:r})=>(s>0?s--:r<e.doc.length&&r++,{from:s,to:r}))),n=mn(e.selection,s=>{let r;if(i.lineWrapping){let o=i.lineBlockAt(s.head),a=i.coordsAtPos(s.head,s.assoc||1);a&&(r=o.bottom+i.documentTop-a.bottom+i.defaultLineHeight/2)}return i.moveVertically(s,!0,r)}).map(t);return i.dispatch({changes:t,selection:n,scrollIntoView:!0,userEvent:"delete.line"}),!0};function Ug(i,e){if(/\(\)|\[\]|\{\}/.test(i.sliceDoc(e-1,e+1)))return{from:e,to:e};let t=Je(i).resolveInner(e),n=t.childBefore(e),s=t.childAfter(e),r;return n&&s&&n.to<=e&&s.from>=e&&(r=n.type.prop(U.closedBy))&&r.indexOf(s.name)>-1&&i.doc.lineAt(n.to).from==i.doc.lineAt(s.from).from&&!/\S/.test(i.sliceDoc(n.to,s.from))?{from:n.to,to:s.from}:null}const il=_c(!1),$g=_c(!0);function _c(i){return({state:e,dispatch:t})=>{if(e.readOnly)return!1;let n=e.changeByRange(s=>{let{from:r,to:o}=s,a=e.doc.lineAt(r),l=!i&&r==o&&Ug(e,r);i&&(r=o=(o<=a.to?a:e.doc.lineAt(o)).to);let h=new ws(e,{simulateBreak:r,simulateDoubleBreak:!!l}),c=Uh(h,r);for(c==null&&(c=hs(/^\s*/.exec(e.doc.lineAt(r).text)[0],e.tabSize));o<a.to&&/\s/.test(a.text[o-a.from]);)o++;l?{from:r,to:o}=l:r>a.from&&r<a.from+100&&!/\S/.test(a.text.slice(0,r))&&(r=a.from);let d=["",Qi(e,c)];return l&&d.push(Qi(e,h.lineIndent(a.from,-1))),{changes:{from:r,to:o,insert:j.of(d)},range:b.cursor(r+1+d[1].length)}});return t(e.update(n,{scrollIntoView:!0,userEvent:"input"})),!0}}function So(i,e){let t=-1;return i.changeByRange(n=>{let s=[];for(let o=n.from;o<=n.to;){let a=i.doc.lineAt(o);a.number>t&&(n.empty||n.to>a.from)&&(e(a,s,n),t=a.number),o=a.to+1}let r=i.changes(s);return{changes:s,range:b.range(r.mapPos(n.anchor,1),r.mapPos(n.head,1))}})}const Kg=({state:i,dispatch:e})=>{if(i.readOnly)return!1;let t=Object.create(null),n=new ws(i,{overrideIndentation:r=>{let o=t[r];return o??-1}}),s=So(i,(r,o,a)=>{let l=Uh(n,r.from);if(l==null)return;/\S/.test(r.text)||(l=0);let h=/^\s*/.exec(r.text)[0],c=Qi(i,l);(h!=c||a.from<r.from+h.length)&&(t[r.from]=l,o.push({from:r.from,to:r.from+h.length,insert:c}))});return s.changes.empty||e(i.update(s,{userEvent:"indent"})),!0},Pc=({state:i,dispatch:e})=>i.readOnly?!1:(e(i.update(So(i,(t,n)=>{n.push({from:t.from,insert:i.facet(bs)})}),{userEvent:"input.indent"})),!0),Nc=({state:i,dispatch:e})=>i.readOnly?!1:(e(i.update(So(i,(t,n)=>{let s=/^\s*/.exec(t.text)[0];if(!s)return;let r=hs(s,i.tabSize),o=0,a=Qi(i,Math.max(0,r-Xi(i)));for(;o<s.length&&o<a.length&&s.charCodeAt(o)==a.charCodeAt(o);)o++;n.push({from:t.from+o,to:t.from+s.length,insert:a.slice(o)})}),{userEvent:"delete.dedent"})),!0),Gg=i=>(i.setTabFocusMode(),!0),Yg=[{key:"Ctrl-b",run:uc,shift:kc,preventDefault:!0},{key:"Ctrl-f",run:fc,shift:xc},{key:"Ctrl-p",run:gc,shift:Ac},{key:"Ctrl-n",run:yc,shift:Tc},{key:"Ctrl-a",run:pg,shift:Tg},{key:"Ctrl-e",run:mg,shift:Mg},{key:"Ctrl-d",run:Dc},{key:"Ctrl-h",run:Ur},{key:"Ctrl-k",run:_g},{key:"Ctrl-Alt-h",run:Ec},{key:"Ctrl-o",run:Wg},{key:"Ctrl-t",run:qg},{key:"Ctrl-v",run:jr}],Jg=[{key:"ArrowLeft",run:uc,shift:kc,preventDefault:!0},{key:"Mod-ArrowLeft",mac:"Alt-ArrowLeft",run:rg,shift:bg,preventDefault:!0},{mac:"Cmd-ArrowLeft",run:ug,shift:Cg,preventDefault:!0},{key:"ArrowRight",run:fc,shift:xc,preventDefault:!0},{key:"Mod-ArrowRight",mac:"Alt-ArrowRight",run:og,shift:wg,preventDefault:!0},{mac:"Cmd-ArrowRight",run:fg,shift:Ag,preventDefault:!0},{key:"ArrowUp",run:gc,shift:Ac,preventDefault:!0},{mac:"Cmd-ArrowUp",run:Za,shift:tl},{mac:"Ctrl-ArrowUp",run:Ja,shift:Xa},{key:"ArrowDown",run:yc,shift:Tc,preventDefault:!0},{mac:"Cmd-ArrowDown",run:el,shift:nl},{mac:"Ctrl-ArrowDown",run:jr,shift:Qa},{key:"PageUp",run:Ja,shift:Xa},{key:"PageDown",run:jr,shift:Qa},{key:"Home",run:dg,shift:Sg,preventDefault:!0},{key:"Mod-Home",run:Za,shift:tl},{key:"End",run:cg,shift:xg,preventDefault:!0},{key:"Mod-End",run:el,shift:nl},{key:"Enter",run:il,shift:il},{key:"Mod-a",run:Og},{key:"Backspace",run:Ur,shift:Ur,preventDefault:!0},{key:"Delete",run:Dc,preventDefault:!0},{key:"Mod-Backspace",mac:"Alt-Backspace",run:Ec,preventDefault:!0},{key:"Mod-Delete",mac:"Alt-Delete",run:Bg,preventDefault:!0},{mac:"Mod-Backspace",run:Pg,preventDefault:!0},{mac:"Mod-Delete",run:Ng,preventDefault:!0}].concat(Yg.map(i=>({mac:i.key,run:i.run,shift:i.shift}))),Xg=[{key:"Alt-ArrowLeft",mac:"Ctrl-ArrowLeft",run:lg,shift:vg},{key:"Alt-ArrowRight",mac:"Ctrl-ArrowRight",run:hg,shift:kg},{key:"Alt-ArrowUp",run:Hg},{key:"Shift-Alt-ArrowUp",run:Vg},{key:"Alt-ArrowDown",run:Fg},{key:"Shift-Alt-ArrowDown",run:zg},{key:"Mod-Alt-ArrowUp",run:Rg},{key:"Mod-Alt-ArrowDown",run:Eg},{key:"Escape",run:Lg},{key:"Mod-Enter",run:$g},{key:"Alt-l",mac:"Ctrl-l",run:Ig},{key:"Mod-i",run:Dg,preventDefault:!0},{key:"Mod-[",run:Nc},{key:"Mod-]",run:Pc},{key:"Mod-Alt-\\",run:Kg},{key:"Shift-Mod-k",run:jg},{key:"Shift-Mod-\\",run:yg},{key:"Mod-/",run:Hm},{key:"Alt-A",mac:"Ctrl-A",run:Vm},{key:"Ctrl-m",mac:"Shift-Alt-m",run:Gg}].concat(Jg),Qg={key:"Tab",run:Pc,shift:Nc};class sl{constructor(e,t,n){this.from=e,this.to=t,this.diagnostic=n}}class Pt{constructor(e,t,n){this.diagnostics=e,this.panel=t,this.selected=n}static init(e,t,n){let s=n.facet(ct).markerFilter;s&&(e=s(e,n));let r=e.slice().sort((f,p)=>f.from-p.from||f.to-p.to),o=new At,a=[],l=0,h=n.doc.iter(),c=0,d=n.doc.length;for(let f=0;;){let p=f==r.length?null:r[f];if(!p&&!a.length)break;let m,g;if(a.length)m=l,g=a.reduce((v,I)=>Math.min(v,I.to),p&&p.from>m?p.from:1e8);else{if(m=p.from,m>d)break;g=p.to,a.push(p),f++}for(;f<r.length;){let v=r[f];if(v.from==m&&(v.to>v.from||v.to==m))a.push(v),f++,g=Math.min(v.to,g);else{g=Math.min(v.from,g);break}}g=Math.min(g,d);let y=!1;if(a.some(v=>v.from==m&&(v.to==g||g==d))&&(y=m==g,!y&&g-m<10)){let v=m-(c+h.value.length);v>0&&(h.next(v),c=m);for(let I=m;;){if(I>=g){y=!0;break}if(!h.lineBreak&&c+h.value.length>I)break;I=c+h.value.length,c+=h.value.length,h.next()}}let x=jc(a);if(y)o.add(m,m,_.widget({widget:new l0(x),diagnostics:a.slice()}));else{let v=a.reduce((I,S)=>S.markClass?I+" "+S.markClass:I,"");o.add(m,g,_.mark({class:"cm-lintRange cm-lintRange-"+x+v,diagnostics:a.slice(),inclusiveEnd:a.some(I=>I.to>g)}))}if(l=g,l==d)break;for(let v=0;v<a.length;v++)a[v].to<=l&&a.splice(v--,1)}let u=o.finish();return new Pt(u,t,Rt(u))}}function Rt(i,e=null,t=0){let n=null;return i.between(t,1e9,(s,r,{spec:o})=>{if(!(e&&o.diagnostics.indexOf(e)<0))if(!n)n=new sl(s,r,e||o.diagnostics[0]);else{if(o.diagnostics.indexOf(n.diagnostic)<0)return!1;n=new sl(n.from,r,n.diagnostic)}}),n}function Wc(i,e){let t=e.pos,n=e.end||t,s=i.state.facet(ct).hideOn(i,t,n);if(s!=null)return s;let r=i.startState.doc.lineAt(e.pos);return!!(i.effects.some(o=>o.is(As))||i.changes.touchesRange(r.from,Math.max(r.to,n)))}function qc(i,e){return i.field(Pe,!1)?e:e.concat(N.appendConfig.of(Gc))}function Zg(i,e){return{effects:qc(i,[As.of(e)])}}const As=N.define(),Co=N.define(),Hc=N.define(),Pe=me.define({create(){return new Pt(_.none,null,null)},update(i,e){if(e.docChanged&&i.diagnostics.size){let t=i.diagnostics.map(e.changes),n=null,s=i.panel;if(i.selected){let r=e.changes.mapPos(i.selected.from,1);n=Rt(t,i.selected.diagnostic,r)||Rt(t,null,r)}!t.size&&s&&e.state.facet(ct).autoPanel&&(s=null),i=new Pt(t,s,n)}for(let t of e.effects)if(t.is(As)){let n=e.state.facet(ct).autoPanel?t.value.length?Yn.open:null:i.panel;i=Pt.init(t.value,n,e.state)}else t.is(Co)?i=new Pt(i.diagnostics,t.value?Yn.open:null,i.selected):t.is(Hc)&&(i=new Pt(i.diagnostics,i.panel,t.value));return i},provide:i=>[Un.from(i,e=>e.panel),R.decorations.from(i,e=>e.diagnostics)]}),e0=_.mark({class:"cm-lintRange cm-lintRange-active"});function t0(i,e,t){let{diagnostics:n}=i.state.field(Pe),s,r=-1,o=-1;n.between(e-(t<0?1:0),e+(t>0?1:0),(l,h,{spec:c})=>{if(e>=l&&e<=h&&(l==h||(e>l||t>0)&&(e<h||t<0)))return s=c.diagnostics,r=l,o=h,!1});let a=i.state.facet(ct).tooltipFilter;return s&&a&&(s=a(s,i.state)),s?{pos:r,end:o,above:!0,create(){return{dom:Fc(i,s)}}}:null}function Fc(i,e){return G("ul",{class:"cm-tooltip-lint"},e.map(t=>zc(i,t,!1)))}const n0=i=>{let e=i.state.field(Pe,!1);(!e||!e.panel)&&i.dispatch({effects:qc(i.state,[Co.of(!0)])});let t=ys(i,Yn.open);return t&&t.dom.querySelector(".cm-panel-lint ul").focus(),!0},rl=i=>{let e=i.state.field(Pe,!1);return!e||!e.panel?!1:(i.dispatch({effects:Co.of(!1)}),!0)},i0=i=>{let e=i.state.field(Pe,!1);if(!e)return!1;let t=i.state.selection.main,n=Rt(e.diagnostics,null,t.to+1);return!n&&(n=Rt(e.diagnostics,null,0),!n||n.from==t.from&&n.to==t.to)?!1:(i.dispatch({selection:{anchor:n.from,head:n.to},scrollIntoView:!0}),Yf(i,n.from,1,{tooltip:Kc,until:s=>s.docChanged||s.newSelection.main.head<n.from||s.newSelection.main.head>n.to}),!0)},s0=[{key:"Mod-Shift-m",run:n0,preventDefault:!0},{key:"F8",run:i0}],r0=ce.fromClass(class{constructor(i){this.view=i,this.timeout=-1,this.set=!0;let{delay:e}=i.state.facet(ct);this.lintTime=Date.now()+e,this.run=this.run.bind(this),this.timeout=setTimeout(this.run,e)}run(){clearTimeout(this.timeout);let i=Date.now();if(i<this.lintTime-10)this.timeout=setTimeout(this.run,this.lintTime-i);else{this.set=!1;let{state:e}=this.view,{sources:t}=e.facet(ct);t.length&&o0(t.map(n=>Promise.resolve(n(this.view))),n=>{this.view.state.doc==e.doc&&this.view.dispatch(Zg(this.view.state,n.reduce((s,r)=>s.concat(r))))},n=>{Ce(this.view.state,n)})}}update(i){let e=i.state.facet(ct);(i.docChanged||e!=i.startState.facet(ct)||e.needsRefresh&&e.needsRefresh(i))&&(this.lintTime=Date.now()+e.delay,this.set||(this.set=!0,this.timeout=setTimeout(this.run,e.delay)))}force(){this.set&&(this.lintTime=Date.now(),this.run())}destroy(){clearTimeout(this.timeout)}});function o0(i,e,t){let n=[],s=-1;for(let r of i)r.then(o=>{n.push(o),clearTimeout(s),n.length==i.length?e(n):s=setTimeout(()=>e(n),200)},t)}const ct=D.define({combine(i){return{sources:i.map(e=>e.source).filter(e=>e!=null),...bt(i.map(e=>e.config),{delay:750,markerFilter:null,tooltipFilter:null,needsRefresh:null,hideOn:()=>null},{delay:Math.max,markerFilter:ol,tooltipFilter:ol,needsRefresh:(e,t)=>e?t?n=>e(n)||t(n):e:t,hideOn:(e,t)=>e?t?(n,s,r)=>e(n,s,r)||t(n,s,r):e:t,autoPanel:(e,t)=>e||t})}}});function ol(i,e){return i?e?(t,n)=>e(i(t,n),n):i:e}function a0(i,e={}){return[ct.of({source:i,config:e}),r0,Gc]}function Vc(i){let e=[];if(i)e:for(let{name:t}of i){for(let n=0;n<t.length;n++){let s=t[n];if(/[a-zA-Z]/.test(s)&&!e.some(r=>r.toLowerCase()==s.toLowerCase())){e.push(s);continue e}}e.push("")}return e}function zc(i,e,t){var n;let s=t?Vc(e.actions):[];return G("li",{class:"cm-diagnostic cm-diagnostic-"+e.severity},G("span",{class:"cm-diagnosticText"},e.renderMessage?e.renderMessage(i):e.message),(n=e.actions)===null||n===void 0?void 0:n.map((r,o)=>{let a=!1,l=f=>{if(f.preventDefault(),a)return;a=!0;let p=Rt(i.state.field(Pe).diagnostics,e);p&&r.apply(i,p.from,p.to)},{name:h}=r,c=s[o]?h.indexOf(s[o]):-1,d=c<0?h:[h.slice(0,c),G("u",h.slice(c,c+1)),h.slice(c+1)],u=r.markClass?" "+r.markClass:"";return G("button",{type:"button",class:"cm-diagnosticAction"+u,onclick:l,onmousedown:l,"aria-label":` Action: ${h}${c<0?"":` (access key "${s[o]})"`}.`},d)}),e.source&&G("div",{class:"cm-diagnosticSource"},e.source))}class l0 extends Xn{constructor(e){super(),this.sev=e}eq(e){return e.sev==this.sev}toDOM(){return G("span",{class:"cm-lintPoint cm-lintPoint-"+this.sev})}}class al{constructor(e,t){this.diagnostic=t,this.id="item_"+Math.floor(Math.random()*4294967295).toString(16),this.dom=zc(e,t,!0),this.dom.id=this.id,this.dom.setAttribute("role","option")}}class Yn{constructor(e){this.view=e,this.items=[];let t=s=>{if(!(s.ctrlKey||s.altKey||s.metaKey)){if(s.keyCode==27)rl(this.view),this.view.focus();else if(s.keyCode==38||s.keyCode==33)this.moveSelection((this.selectedIndex-1+this.items.length)%this.items.length);else if(s.keyCode==40||s.keyCode==34)this.moveSelection((this.selectedIndex+1)%this.items.length);else if(s.keyCode==36)this.moveSelection(0);else if(s.keyCode==35)this.moveSelection(this.items.length-1);else if(s.keyCode==13)this.view.focus();else if(s.keyCode>=65&&s.keyCode<=90&&this.selectedIndex>=0){let{diagnostic:r}=this.items[this.selectedIndex],o=Vc(r.actions);for(let a=0;a<o.length;a++)if(o[a].toUpperCase().charCodeAt(0)==s.keyCode){let l=Rt(this.view.state.field(Pe).diagnostics,r);l&&r.actions[a].apply(e,l.from,l.to)}}else return;s.preventDefault()}},n=s=>{for(let r=0;r<this.items.length;r++)this.items[r].dom.contains(s.target)&&this.moveSelection(r)};this.list=G("ul",{tabIndex:0,role:"listbox","aria-label":this.view.state.phrase("Diagnostics"),onkeydown:t,onclick:n}),this.dom=G("div",{class:"cm-panel-lint"},this.list,G("button",{type:"button",name:"close","aria-label":this.view.state.phrase("close"),onclick:()=>rl(this.view)},"×")),this.update()}get selectedIndex(){let e=this.view.state.field(Pe).selected;if(!e)return-1;for(let t=0;t<this.items.length;t++)if(this.items[t].diagnostic==e.diagnostic)return t;return-1}update(){let{diagnostics:e,selected:t}=this.view.state.field(Pe),n=0,s=!1,r=null,o=new Set;for(e.between(0,this.view.state.doc.length,(a,l,{spec:h})=>{for(let c of h.diagnostics){if(o.has(c))continue;o.add(c);let d=-1,u;for(let f=n;f<this.items.length;f++)if(this.items[f].diagnostic==c){d=f;break}d<0?(u=new al(this.view,c),this.items.splice(n,0,u),s=!0):(u=this.items[d],d>n&&(this.items.splice(n,d-n),s=!0)),t&&u.diagnostic==t.diagnostic?u.dom.hasAttribute("aria-selected")||(u.dom.setAttribute("aria-selected","true"),r=u):u.dom.hasAttribute("aria-selected")&&u.dom.removeAttribute("aria-selected"),n++}});n<this.items.length&&!(this.items.length==1&&this.items[0].diagnostic.from<0);)s=!0,this.items.pop();this.items.length==0&&(this.items.push(new al(this.view,{from:-1,to:-1,severity:"info",message:this.view.state.phrase("No diagnostics")})),s=!0),r?(this.list.setAttribute("aria-activedescendant",r.id),this.view.requestMeasure({key:this,read:()=>({sel:r.dom.getBoundingClientRect(),panel:this.list.getBoundingClientRect()}),write:({sel:a,panel:l})=>{let h=l.height/this.list.offsetHeight;a.top<l.top?this.list.scrollTop-=(l.top-a.top)/h:a.bottom>l.bottom&&(this.list.scrollTop+=(a.bottom-l.bottom)/h)}})):this.selectedIndex<0&&this.list.removeAttribute("aria-activedescendant"),s&&this.sync()}sync(){let e=this.list.firstChild;function t(){let n=e;e=n.nextSibling,n.remove()}for(let n of this.items)if(n.dom.parentNode==this.list){for(;e!=n.dom;)t();e=n.dom.nextSibling}else this.list.insertBefore(n.dom,e);for(;e;)t()}moveSelection(e){if(this.selectedIndex<0)return;let t=this.view.state.field(Pe),n=Rt(t.diagnostics,this.items[e].diagnostic);n&&this.view.dispatch({selection:{anchor:n.from,head:n.to},scrollIntoView:!0,effects:Hc.of(n)})}static open(e){return new Yn(e)}}function _i(i,e='viewBox="0 0 40 40"'){return`url('data:image/svg+xml,<svg xmlns="http://www.w3.org/2000/svg" ${e}>${encodeURIComponent(i)}</svg>')`}function Ci(i){return _i(`<path d="m0 2.5 l2 -1.5 l1 0 l2 1.5 l1 0" stroke="${i}" fill="none" stroke-width=".7"/>`,'width="6" height="3"')}const h0=R.baseTheme({".cm-diagnostic":{padding:"3px 6px 3px 8px",marginLeft:"-1px",display:"block",whiteSpace:"pre-wrap"},".cm-diagnostic-error":{borderLeft:"5px solid #d11"},".cm-diagnostic-warning":{borderLeft:"5px solid orange"},".cm-diagnostic-info":{borderLeft:"5px solid #999"},".cm-diagnostic-hint":{borderLeft:"5px solid #66d"},".cm-diagnosticAction":{font:"inherit",border:"none",padding:"2px 4px",backgroundColor:"#444",color:"white",borderRadius:"3px",marginLeft:"8px",cursor:"pointer"},".cm-diagnosticSource":{fontSize:"70%",opacity:.7},".cm-lintRange":{backgroundPosition:"left bottom",backgroundRepeat:"repeat-x",paddingBottom:"0.7px"},".cm-lintRange-error":{backgroundImage:Ci("#f11")},".cm-lintRange-warning":{backgroundImage:Ci("orange")},".cm-lintRange-info":{backgroundImage:Ci("#999")},".cm-lintRange-hint":{backgroundImage:Ci("#66d")},".cm-lintRange-active":{backgroundColor:"#ffdd9980"},".cm-tooltip-lint":{padding:0,margin:0},".cm-lintPoint":{position:"relative","&:after":{content:'""',position:"absolute",bottom:0,left:"-2px",borderLeft:"3px solid transparent",borderRight:"3px solid transparent",borderBottom:"4px solid #d11"}},".cm-lintPoint-warning":{"&:after":{borderBottomColor:"orange"}},".cm-lintPoint-info":{"&:after":{borderBottomColor:"#999"}},".cm-lintPoint-hint":{"&:after":{borderBottomColor:"#66d"}},".cm-panel.cm-panel-lint":{position:"relative","& ul":{maxHeight:"100px",overflowY:"auto","& [aria-selected]":{backgroundColor:"#ddd","& u":{textDecoration:"underline"}},"&:focus [aria-selected]":{background_fallback:"#bdf",backgroundColor:"Highlight",color_fallback:"white",color:"HighlightText"},"& u":{textDecoration:"none"},padding:0,margin:0},"& [name=close]":{position:"absolute",top:"0",right:"2px",background:"inherit",border:"none",font:"inherit",padding:0,margin:0}},"&dark .cm-lintRange-active":{backgroundColor:"#86714a80"},"&dark .cm-panel.cm-panel-lint ul":{"& [aria-selected]":{backgroundColor:"#2e343e"}}});function c0(i){return i=="error"?4:i=="warning"?3:i=="info"?2:1}function jc(i){let e="hint",t=1;for(let n of i){let s=c0(n.severity);s>t&&(t=s,e=n.severity)}return e}class Uc extends gt{constructor(e){super(),this.diagnostics=e,this.severity=jc(e)}toDOM(e){let t=document.createElement("div");t.className="cm-lint-marker cm-lint-marker-"+this.severity;let n=this.diagnostics,s=e.state.facet(Ts).tooltipFilter;return s&&(n=s(n,e.state)),n.length&&(t.onmouseover=()=>u0(e,t,n)),t}}function d0(i,e){let t=n=>{let s=e.getBoundingClientRect();if(!(n.clientX>s.left-10&&n.clientX<s.right+10&&n.clientY>s.top-10&&n.clientY<s.bottom+10)){for(let r=n.target;r;r=r.parentNode)if(r.nodeType==1&&r.classList.contains("cm-tooltip-lint"))return;window.removeEventListener("mousemove",t),i.state.field($c)&&i.dispatch({effects:Ao.of(null)})}};window.addEventListener("mousemove",t)}function u0(i,e,t){function n(){let o=i.elementAtHeight(e.getBoundingClientRect().top+5-i.documentTop);i.coordsAtPos(o.from)&&i.dispatch({effects:Ao.of({pos:o.from,above:!1,clip:!1,create(){return{dom:Fc(i,t),getCoords:()=>e.getBoundingClientRect()}}})}),e.onmouseout=e.onmousemove=null,d0(i,e)}let{hoverTime:s}=i.state.facet(Ts),r=setTimeout(n,s);e.onmouseout=()=>{clearTimeout(r),e.onmouseout=e.onmousemove=null},e.onmousemove=()=>{clearTimeout(r),r=setTimeout(n,s)}}function f0(i,e){let t=Object.create(null);for(let s of e){let r=i.lineAt(s.from);(t[r.from]||(t[r.from]=[])).push(s)}let n=[];for(let s in t)n.push(new Uc(t[s]).range(+s));return H.of(n,!0)}const p0=np({class:"cm-gutter-lint",markers:i=>i.state.field($r),widgetMarker:(i,e,t)=>{let n=[];return i.state.field($r).between(t.from,t.to,(s,r,o)=>{s>t.from&&s<t.to&&n.push(...o.diagnostics)}),n.length?new Uc(n):null}}),$r=me.define({create(){return H.empty},update(i,e){i=i.map(e.changes);let t=e.state.facet(Ts).markerFilter;for(let n of e.effects)if(n.is(As)){let s=n.value;t&&(s=t(s||[],e.state)),i=f0(e.state.doc,s.slice(0))}return i}}),Ao=N.define(),$c=me.define({create(){return null},update(i,e){return i&&e.docChanged&&(i=Wc(e,i)?null:{...i,pos:e.changes.mapPos(i.pos)}),e.effects.reduce((t,n)=>n.is(Ao)?n.value:t,i)},provide:i=>ms.from(i)}),m0=R.baseTheme({".cm-gutter-lint":{width:"1.4em","& .cm-gutterElement":{padding:".2em"}},".cm-lint-marker":{width:"1em",height:"1em"},".cm-lint-marker-info":{content:_i('<path fill="#aaf" stroke="#77e" stroke-width="6" stroke-linejoin="round" d="M5 5L35 5L35 35L5 35Z"/>')},".cm-lint-marker-warning":{content:_i('<path fill="#fe8" stroke="#fd7" stroke-width="6" stroke-linejoin="round" d="M20 6L37 35L3 35Z"/>')},".cm-lint-marker-error":{content:_i('<circle cx="20" cy="20" r="15" fill="#f87" stroke="#f43" stroke-width="6"/>')}}),Kc=Bh(t0,{hideOn:Wc}),Gc=[Pe,R.decorations.compute([Pe],i=>{let{selected:e,panel:t}=i.field(Pe);return!e||!t||e.from==e.to?_.none:_.set([e0.range(e.from,e.to)])}),Kc,h0],Ts=D.define({combine(i){return bt(i,{hoverTime:300,markerFilter:null,tooltipFilter:null})}});function g0(i={}){return[Ts.of(i),$r,p0,m0,$c]}const ll=typeof String.prototype.normalize=="function"?i=>i.normalize("NFKD"):i=>i;class fn{constructor(e,t,n=0,s=e.length,r,o){this.test=o,this.value={from:0,to:0,precise:!1},this.done=!1,this.matches=[],this.buffer="",this.bufferPos=0,this.iter=e.iterRange(n,s),this.bufferStart=n,this.normalize=r?a=>r(ll(a)):ll,this.query=this.normalize(t)}peek(){if(this.bufferPos==this.buffer.length){if(this.bufferStart+=this.buffer.length,this.iter.next(),this.iter.done)return-1;this.bufferPos=0,this.buffer=this.iter.value}return _e(this.buffer,this.bufferPos)}next(){for(;this.matches.length;)this.matches.pop();return this.nextOverlapping()}nextOverlapping(){for(;;){let e=this.peek();if(e<0)return this.done=!0,this;let t=Gr(e),n=this.bufferStart+this.bufferPos;this.bufferPos+=rt(e);let s=this.normalize(t);if(s.length)for(let r=0,o=n,a=!0;;r++){let l=s.charCodeAt(r),h=this.match(l,o,a,this.bufferPos+this.bufferStart,r==s.length-1);if(h)return this.value=h,this;if(r==s.length-1)break;a&&r<t.length&&t.charCodeAt(r)==l?o++:a=!1}}}match(e,t,n,s,r){let o=null;for(let a=0;a<this.matches.length;){let l=this.matches[a],h=!1;this.query.charCodeAt(l.index)==e&&(l.index==this.query.length-1?o={from:l.from,to:s,precise:r&&l.precise}:(l.index++,h=!0)),h?a++:this.matches.splice(a,1)}return this.query.charCodeAt(0)==e&&(this.query.length==1?o={from:t,to:s,precise:n&&r}:this.matches.push({from:t,index:1,precise:n})),o&&this.test&&!this.test(o.from,o.to,this.buffer,this.bufferStart)&&(o=null),o}}typeof Symbol<"u"&&(fn.prototype[Symbol.iterator]=function(){return this});const Yc={from:-1,to:-1,match:/.*/.exec(""),precise:!0},To="gm"+(/x/.unicode==null?"":"u");class Jc{constructor(e,t,n,s=0,r=e.length){if(this.text=e,this.to=r,this.curLine="",this.done=!1,this.value=Yc,/\\[sWDnr]|\n|\r|\[\^/.test(t))return new Xc(e,t,n,s,r);this.re=new RegExp(t,To+(n!=null&&n.ignoreCase?"i":"")),this.test=n==null?void 0:n.test,this.iter=e.iter();let o=e.lineAt(s);this.curLineStart=o.from,this.matchPos=ns(e,s),this.getLine(this.curLineStart)}getLine(e){this.iter.next(e),this.iter.lineBreak?this.curLine="":(this.curLine=this.iter.value,this.curLineStart+this.curLine.length>this.to&&(this.curLine=this.curLine.slice(0,this.to-this.curLineStart)),this.iter.next())}nextLine(){this.curLineStart=this.curLineStart+this.curLine.length+1,this.curLineStart>this.to?this.curLine="":this.getLine(0)}next(){for(let e=this.matchPos-this.curLineStart;;){this.re.lastIndex=e;let t=this.matchPos<=this.to&&this.re.exec(this.curLine);if(t){let n=this.curLineStart+t.index,s=n+t[0].length;if(this.matchPos=ns(this.text,s+(n==s?1:0)),n==this.curLineStart+this.curLine.length&&this.nextLine(),(n<s||n>this.value.to)&&(!this.test||this.test(n,s,t)))return this.value={from:n,to:s,precise:!0,match:t},this;e=this.matchPos-this.curLineStart}else if(this.curLineStart+this.curLine.length<this.to)this.nextLine(),e=0;else return this.done=!0,this}}}const nr=new WeakMap;class sn{constructor(e,t){this.from=e,this.text=t}get to(){return this.from+this.text.length}static get(e,t,n){let s=nr.get(e);if(!s||s.from>=n||s.to<=t){let a=new sn(t,e.sliceString(t,n));return nr.set(e,a),a}if(s.from==t&&s.to==n)return s;let{text:r,from:o}=s;return o>t&&(r=e.sliceString(t,o)+r,o=t),s.to<n&&(r+=e.sliceString(s.to,n)),nr.set(e,new sn(o,r)),new sn(t,r.slice(t-o,n-o))}}class Xc{constructor(e,t,n,s,r){this.text=e,this.to=r,this.done=!1,this.value=Yc,this.matchPos=ns(e,s),this.re=new RegExp(t,To+(n!=null&&n.ignoreCase?"i":"")),this.test=n==null?void 0:n.test,this.flat=sn.get(e,s,this.chunkEnd(s+5e3))}chunkEnd(e){return e>=this.to?this.to:this.text.lineAt(e).to}next(){for(;;){let e=this.re.lastIndex=this.matchPos-this.flat.from,t=this.re.exec(this.flat.text);if(t&&!t[0]&&t.index==e&&(this.re.lastIndex=e+1,t=this.re.exec(this.flat.text)),t){let n=this.flat.from+t.index,s=n+t[0].length;if((this.flat.to>=this.to||t.index+t[0].length<=this.flat.text.length-10)&&(!this.test||this.test(n,s,t)))return this.value={from:n,to:s,precise:!0,match:t},this.matchPos=ns(this.text,s+(n==s?1:0)),this}if(this.flat.to==this.to)return this.done=!0,this;this.flat=sn.get(this.text,this.flat.from,this.chunkEnd(this.flat.from+this.flat.text.length*2))}}}typeof Symbol<"u"&&(Jc.prototype[Symbol.iterator]=Xc.prototype[Symbol.iterator]=function(){return this});function y0(i){try{return new RegExp(i,To),!0}catch{return!1}}function ns(i,e){if(e>=i.length)return e;let t=i.lineAt(e),n;for(;e<t.to&&(n=t.text.charCodeAt(e-t.from))>=56320&&n<57344;)e++;return e}const b0=i=>{let e=Qf(i,"cm-goto-line");if(e){let o=e.dom.querySelector("input[type=text]");return o&&o.select(),!0}let{state:t}=i,n=String(t.doc.lineAt(i.state.selection.main.head).number),{close:s,result:r}=Xf(i,{class:"cm-goto-line",label:t.phrase("Go to line"),input:{type:"text",name:"line",value:n},focus:!0,submitLabel:t.phrase("go")});return r.then(o=>{let a=o&&/^([+-])?(\d+)?(:\d+)?(%)?$/.exec(o.elements.line.value);if(!a){i.dispatch({effects:s});return}let l=t.doc.lineAt(t.selection.main.head),[,h,c,d,u]=a,f=d?+d.slice(1):0,p=c?+c:l.number;if(c&&u){let y=p/100;h&&(y=y*(h=="-"?-1:1)+l.number/t.doc.lines),p=Math.round(t.doc.lines*y)}else c&&h&&(p=p*(h=="-"?-1:1)+l.number);let m=t.doc.line(Math.max(1,Math.min(t.doc.lines,p))),g=b.cursor(m.from+Math.max(0,Math.min(f,m.length)));i.dispatch({effects:[s,R.scrollIntoView(g.from,{y:"center"})],selection:g})}),!0},w0={highlightWordAroundCursor:!1,minSelectionLength:1,maxMatches:100,wholeWords:!1},v0=D.define({combine(i){return bt(i,w0,{highlightWordAroundCursor:(e,t)=>e||t,minSelectionLength:Math.min,maxMatches:Math.min})}});function k0(i){return[T0,A0]}const x0=_.mark({class:"cm-selectionMatch"}),S0=_.mark({class:"cm-selectionMatch cm-selectionMatch-main"});function hl(i,e,t,n){return(t==0||i(e.sliceDoc(t-1,t))!=Z.Word)&&(n==e.doc.length||i(e.sliceDoc(n,n+1))!=Z.Word)}function C0(i,e,t,n){return i(e.sliceDoc(t,t+1))==Z.Word&&i(e.sliceDoc(n-1,n))==Z.Word}const A0=ce.fromClass(class{constructor(i){this.decorations=this.getDeco(i)}update(i){(i.selectionSet||i.docChanged||i.viewportChanged)&&(this.decorations=this.getDeco(i.view))}getDeco(i){let e=i.state.facet(v0),{state:t}=i,n=t.selection;if(n.ranges.length>1)return _.none;let s=n.main,r,o=null;if(s.empty){if(!e.highlightWordAroundCursor)return _.none;let l=t.wordAt(s.head);if(!l)return _.none;o=t.charCategorizer(s.head),r=t.sliceDoc(l.from,l.to)}else{let l=s.to-s.from;if(l<e.minSelectionLength||l>200)return _.none;if(e.wholeWords){if(r=t.sliceDoc(s.from,s.to),o=t.charCategorizer(s.head),!(hl(o,t,s.from,s.to)&&C0(o,t,s.from,s.to)))return _.none}else if(r=t.sliceDoc(s.from,s.to),!r)return _.none}let a=[];for(let l of i.visibleRanges){let h=new fn(t.doc,r,l.from,l.to);for(;!h.next().done;){let{from:c,to:d}=h.value;if((!o||hl(o,t,c,d))&&(s.empty&&c<=s.from&&d>=s.to?a.push(S0.range(c,d)):(c>=s.to||d<=s.from)&&a.push(x0.range(c,d)),a.length>e.maxMatches))return _.none}}return _.set(a)}},{decorations:i=>i.decorations}),T0=R.baseTheme({".cm-selectionMatch":{backgroundColor:"#99ff7780"},".cm-searchMatch .cm-selectionMatch":{backgroundColor:"transparent"}}),M0=({state:i,dispatch:e})=>{let{selection:t}=i,n=b.create(t.ranges.map(s=>i.wordAt(s.head)||b.cursor(s.head)),t.mainIndex);return n.eq(t)?!1:(e(i.update({selection:n})),!0)};function O0(i,e){let{main:t,ranges:n}=i.selection,s=i.wordAt(t.head),r=s&&s.from==t.from&&s.to==t.to;for(let o=!1,a=new fn(i.doc,e,n[n.length-1].to);;)if(a.next(),a.done){if(o)return null;a=new fn(i.doc,e,0,Math.max(0,n[n.length-1].from-1)),o=!0}else{if(o&&n.some(l=>l.from==a.value.from))continue;if(r){let l=i.wordAt(a.value.from);if(!l||l.from!=a.value.from||l.to!=a.value.to)continue}return a.value}}const I0=({state:i,dispatch:e})=>{let{ranges:t}=i.selection;if(t.some(r=>r.from===r.to))return M0({state:i,dispatch:e});let n=i.sliceDoc(t[0].from,t[0].to);if(i.selection.ranges.some(r=>i.sliceDoc(r.from,r.to)!=n))return!1;let s=O0(i,n);return s?(e(i.update({selection:i.selection.addRange(b.range(s.from,s.to),!1),effects:R.scrollIntoView(s.to)})),!0):!1},gn=D.define({combine(i){return bt(i,{top:!1,caseSensitive:!1,literal:!1,regexp:!1,wholeWord:!1,createPanel:e=>new z0(e),scrollToMatch:e=>R.scrollIntoView(e)})}});class Qc{constructor(e){this.search=e.search,this.caseSensitive=!!e.caseSensitive,this.literal=!!e.literal,this.regexp=!!e.regexp,this.replace=e.replace||"",this.valid=!!this.search&&(!this.regexp||y0(this.search)),this.unquoted=this.unquote(this.search),this.wholeWord=!!e.wholeWord,this.test=e.test}unquote(e){return this.literal?e:e.replace(/\\([nrt\\])/g,(t,n)=>n=="n"?`
`:n=="r"?"\r":n=="t"?"	":"\\")}eq(e){return this.search==e.search&&this.replace==e.replace&&this.caseSensitive==e.caseSensitive&&this.regexp==e.regexp&&this.wholeWord==e.wholeWord&&this.test==e.test}create(){return this.regexp?new _0(this):new E0(this)}getCursor(e,t=0,n){let s=e.doc?e:z.create({doc:e});return n==null&&(n=s.doc.length),this.regexp?Gt(this,s,t,n):Kt(this,s,t,n)}}class Zc{constructor(e){this.spec=e}}function D0(i,e,t){return(n,s,r,o)=>{if(t&&!t(n,s,r,o))return!1;let a=n>=o&&s<=o+r.length?r.slice(n-o,s-o):e.doc.sliceString(n,s);return i(a,e,n,s)}}function Kt(i,e,t,n){let s;return i.wholeWord&&(s=R0(e.doc,e.charCategorizer(e.selection.main.head))),i.test&&(s=D0(i.test,e,s)),new fn(e.doc,i.unquoted,t,n,i.caseSensitive?void 0:r=>r.toLowerCase(),s)}function R0(i,e){return(t,n,s,r)=>((r>t||r+s.length<n)&&(r=Math.max(0,t-2),s=i.sliceString(r,Math.min(i.length,n+2))),(e(is(s,t-r))!=Z.Word||e(ss(s,t-r))!=Z.Word)&&(e(ss(s,n-r))!=Z.Word||e(is(s,n-r))!=Z.Word))}class E0 extends Zc{constructor(e){super(e)}nextMatch(e,t,n){let s=Kt(this.spec,e,n,e.doc.length).nextOverlapping();if(s.done){let r=Math.min(e.doc.length,t+this.spec.unquoted.length);s=Kt(this.spec,e,0,r).nextOverlapping()}return s.done||s.value.from==t&&s.value.to==n?null:s.value}prevMatchInRange(e,t,n){for(let s=n;;){let r=Math.max(t,s-1e4-this.spec.unquoted.length),o=Kt(this.spec,e,r,s),a=null;for(;!o.nextOverlapping().done;)a=o.value;if(a)return a;if(r==t)return null;s-=1e4}}prevMatch(e,t,n){let s=this.prevMatchInRange(e,0,t);return s||(s=this.prevMatchInRange(e,Math.max(0,n-this.spec.unquoted.length),e.doc.length)),s&&(s.from!=t||s.to!=n)?s:null}getReplacement(e){return this.spec.unquote(this.spec.replace)}matchAll(e,t){let n=Kt(this.spec,e,0,e.doc.length),s=[];for(;!n.next().done;){if(s.length>=t)return null;s.push(n.value)}return s}highlight(e,t,n,s){let r=Kt(this.spec,e,Math.max(0,t-this.spec.unquoted.length),Math.min(n+this.spec.unquoted.length,e.doc.length));for(;!r.next().done;)s(r.value.from,r.value.to)}}function L0(i,e,t){return(n,s,r)=>(!t||t(n,s,r))&&i(r[0],e,n,s)}function Gt(i,e,t,n){let s;return i.wholeWord&&(s=B0(e.charCategorizer(e.selection.main.head))),i.test&&(s=L0(i.test,e,s)),new Jc(e.doc,i.search,{ignoreCase:!i.caseSensitive,test:s},t,n)}function is(i,e){return i.slice(he(i,e,!1),e)}function ss(i,e){return i.slice(e,he(i,e))}function B0(i){return(e,t,n)=>!n[0].length||(i(is(n.input,n.index))!=Z.Word||i(ss(n.input,n.index))!=Z.Word)&&(i(ss(n.input,n.index+n[0].length))!=Z.Word||i(is(n.input,n.index+n[0].length))!=Z.Word)}class _0 extends Zc{nextMatch(e,t,n){let s=Gt(this.spec,e,n,e.doc.length).next();return s.done&&(s=Gt(this.spec,e,0,t).next()),s.done?null:s.value}prevMatchInRange(e,t,n){for(let s=1;;s++){let r=Math.max(t,n-s*1e4),o=Gt(this.spec,e,r,n),a=null;for(;!o.next().done;)a=o.value;if(a&&(r==t||a.from>r+10))return a;if(r==t)return null}}prevMatch(e,t,n){return this.prevMatchInRange(e,0,t)||this.prevMatchInRange(e,n,e.doc.length)}getReplacement(e){return this.spec.unquote(this.spec.replace).replace(/\$([$&]|\d+)/g,(t,n)=>{if(n=="&")return e.match[0];if(n=="$")return"$";for(let s=n.length;s>0;s--){let r=+n.slice(0,s);if(r>0&&r<e.match.length)return e.match[r]+n.slice(s)}return t})}matchAll(e,t){let n=Gt(this.spec,e,0,e.doc.length),s=[];for(;!n.next().done;){if(s.length>=t)return null;s.push(n.value)}return s}highlight(e,t,n,s){let r=Gt(this.spec,e,Math.max(0,t-250),Math.min(n+250,e.doc.length));for(;!r.next().done;)s(r.value.from,r.value.to)}}const Jn=N.define(),Mo=N.define(),Tt=me.define({create(i){return new ir(Kr(i).create(),null)},update(i,e){for(let t of e.effects)t.is(Jn)?i=new ir(t.value.create(),i.panel):t.is(Mo)&&(i=new ir(i.query,t.value?Oo:null));return i},provide:i=>Un.from(i,e=>e.panel)});class ir{constructor(e,t){this.query=e,this.panel=t}}const P0=_.mark({class:"cm-searchMatch"}),N0=_.mark({class:"cm-searchMatch cm-searchMatch-selected"}),W0=ce.fromClass(class{constructor(i){this.view=i,this.decorations=this.highlight(i.state.field(Tt))}update(i){let e=i.state.field(Tt);(e!=i.startState.field(Tt)||i.docChanged||i.selectionSet||i.viewportChanged)&&(this.decorations=this.highlight(e))}highlight({query:i,panel:e}){if(!e||!i.spec.valid)return _.none;let{view:t}=this,n=new At;for(let s=0,r=t.visibleRanges,o=r.length;s<o;s++){let{from:a,to:l}=r[s];for(;s<o-1&&l>r[s+1].from-2*250;)l=r[++s].to;i.highlight(t.state,a,l,(h,c)=>{let d=t.state.selection.ranges.some(u=>u.from==h&&u.to==c);n.add(h,c,d?N0:P0)})}return n.finish()}},{decorations:i=>i.decorations});function ni(i){return e=>{let t=e.state.field(Tt,!1);return t&&t.query.spec.valid?i(e,t):nd(e)}}const rs=ni((i,{query:e})=>{let{to:t}=i.state.selection.main,n=e.nextMatch(i.state,t,t);if(!n)return!1;let s=b.single(n.from,n.to),r=i.state.facet(gn);return i.dispatch({selection:s,effects:[Io(i,n),r.scrollToMatch(s.main,i)],userEvent:"select.search"}),td(i),!0}),os=ni((i,{query:e})=>{let{state:t}=i,{from:n}=t.selection.main,s=e.prevMatch(t,n,n);if(!s)return!1;let r=b.single(s.from,s.to),o=i.state.facet(gn);return i.dispatch({selection:r,effects:[Io(i,s),o.scrollToMatch(r.main,i)],userEvent:"select.search"}),td(i),!0}),q0=ni((i,{query:e})=>{let t=e.matchAll(i.state,1e3);return!t||!t.length?!1:(i.dispatch({selection:b.create(t.map(n=>b.range(n.from,n.to))),userEvent:"select.search.matches"}),!0)}),H0=({state:i,dispatch:e})=>{let t=i.selection;if(t.ranges.length>1||t.main.empty)return!1;let{from:n,to:s}=t.main,r=[],o=0;for(let a=new fn(i.doc,i.sliceDoc(n,s));!a.next().done;){if(r.length>1e3)return!1;a.value.from==n&&(o=r.length),r.push(b.range(a.value.from,a.value.to))}return e(i.update({selection:b.create(r,o),userEvent:"select.search.matches"})),!0},cl=ni((i,{query:e})=>{let{state:t}=i,{from:n,to:s}=t.selection.main;if(t.readOnly)return!1;let r=e.nextMatch(t,n,n);if(!r)return!1;let o=r,a=[],l,h,c=[];o.precise?o.from==n&&o.to==s&&(h=t.toText(e.getReplacement(o)),a.push({from:o.from,to:o.to,insert:h}),o=e.nextMatch(t,o.from,o.to),c.push(R.announce.of(t.phrase("replaced match on line $",t.doc.lineAt(n).number)+"."))):o=e.nextMatch(t,o.from,o.to);let d=i.state.changes(a);return o&&(l=b.single(o.from,o.to).map(d),c.push(Io(i,o)),c.push(t.facet(gn).scrollToMatch(l.main,i))),i.dispatch({changes:d,selection:l,effects:c,userEvent:"input.replace"}),!0}),F0=ni((i,{query:e})=>{if(i.state.readOnly)return!1;let t=[];for(let s of e.matchAll(i.state,1e9)){let{from:r,to:o,precise:a}=s;a&&t.push({from:r,to:o,insert:e.getReplacement(s)})}if(!t.length)return!1;let n=i.state.phrase("replaced $ matches",t.length)+".";return i.dispatch({changes:t,effects:R.announce.of(n),userEvent:"input.replace.all"}),!0});function Oo(i){return i.state.facet(gn).createPanel(i)}function Kr(i,e){var t,n,s,r,o;let a=i.selection.main,l=a.empty||a.to>a.from+100?"":i.sliceDoc(a.from,a.to);if(e&&!l)return e;let h=i.facet(gn);return new Qc({search:((t=e==null?void 0:e.literal)!==null&&t!==void 0?t:h.literal)?l:l.replace(/\n/g,"\\n"),caseSensitive:(n=e==null?void 0:e.caseSensitive)!==null&&n!==void 0?n:h.caseSensitive,literal:(s=e==null?void 0:e.literal)!==null&&s!==void 0?s:h.literal,regexp:(r=e==null?void 0:e.regexp)!==null&&r!==void 0?r:h.regexp,wholeWord:(o=e==null?void 0:e.wholeWord)!==null&&o!==void 0?o:h.wholeWord})}function ed(i){let e=ys(i,Oo);return e&&e.dom.querySelector("[main-field]")}function td(i){let e=ed(i);e&&e==i.root.activeElement&&e.select()}const nd=i=>{let e=i.state.field(Tt,!1);if(e&&e.panel){let t=ed(i);if(t&&t!=i.root.activeElement){let n=Kr(i.state,e.query.spec);n.valid&&i.dispatch({effects:Jn.of(n)}),t.focus(),t.select()}}else i.dispatch({effects:[Mo.of(!0),e?Jn.of(Kr(i.state,e.query.spec)):N.appendConfig.of(U0)]});return!0},id=i=>{let e=i.state.field(Tt,!1);if(!e||!e.panel)return!1;let t=ys(i,Oo);return t&&t.dom.contains(i.root.activeElement)&&i.focus(),i.dispatch({effects:Mo.of(!1)}),!0},V0=[{key:"Mod-f",run:nd,scope:"editor search-panel"},{key:"F3",run:rs,shift:os,scope:"editor search-panel",preventDefault:!0},{key:"Mod-g",run:rs,shift:os,scope:"editor search-panel",preventDefault:!0},{key:"Escape",run:id,scope:"editor search-panel"},{key:"Mod-Shift-l",run:H0},{key:"Mod-Alt-g",run:b0},{key:"Mod-d",run:I0,preventDefault:!0}];class z0{constructor(e){this.view=e;let t=this.query=e.state.field(Tt).query.spec;this.commit=this.commit.bind(this),this.searchField=G("input",{value:t.search,placeholder:Le(e,"Find"),"aria-label":Le(e,"Find"),class:"cm-textfield",name:"search",form:"","main-field":"true",onchange:this.commit,onkeyup:this.commit}),this.replaceField=G("input",{value:t.replace,placeholder:Le(e,"Replace"),"aria-label":Le(e,"Replace"),class:"cm-textfield",name:"replace",form:"",onchange:this.commit,onkeyup:this.commit}),this.caseField=G("input",{type:"checkbox",name:"case",form:"",checked:t.caseSensitive,onchange:this.commit}),this.reField=G("input",{type:"checkbox",name:"re",form:"",checked:t.regexp,onchange:this.commit}),this.wordField=G("input",{type:"checkbox",name:"word",form:"",checked:t.wholeWord,onchange:this.commit});function n(s,r,o){return G("button",{class:"cm-button",name:s,onclick:r,type:"button"},o)}this.dom=G("div",{onkeydown:s=>this.keydown(s),class:"cm-search"},[this.searchField,n("next",()=>rs(e),[Le(e,"next")]),n("prev",()=>os(e),[Le(e,"previous")]),n("select",()=>q0(e),[Le(e,"all")]),G("label",null,[this.caseField,Le(e,"match case")]),G("label",null,[this.reField,Le(e,"regexp")]),G("label",null,[this.wordField,Le(e,"by word")]),...e.state.readOnly?[]:[G("br"),this.replaceField,n("replace",()=>cl(e),[Le(e,"replace")]),n("replaceAll",()=>F0(e),[Le(e,"replace all")])],G("button",{name:"close",onclick:()=>id(e),"aria-label":Le(e,"close"),type:"button"},["×"])])}commit(){let e=new Qc({search:this.searchField.value,caseSensitive:this.caseField.checked,regexp:this.reField.checked,wholeWord:this.wordField.checked,replace:this.replaceField.value});e.eq(this.query)||(this.query=e,this.view.dispatch({effects:Jn.of(e)}))}keydown(e){Of(this.view,e,"search-panel")?e.preventDefault():e.keyCode==13&&e.target==this.searchField?(e.preventDefault(),(e.shiftKey?os:rs)(this.view)):e.keyCode==13&&e.target==this.replaceField&&(e.preventDefault(),cl(this.view))}update(e){for(let t of e.transactions)for(let n of t.effects)n.is(Jn)&&!n.value.eq(this.query)&&this.setQuery(n.value)}setQuery(e){this.query=e,this.searchField.value=e.search,this.replaceField.value=e.replace,this.caseField.checked=e.caseSensitive,this.reField.checked=e.regexp,this.wordField.checked=e.wholeWord}mount(){this.searchField.select()}get pos(){return 80}get top(){return this.view.state.facet(gn).top}}function Le(i,e){return i.state.phrase(e)}const Ai=30,Ti=/[\s\.,:;?!]/;function Io(i,{from:e,to:t}){let n=i.state.doc.lineAt(e),s=i.state.doc.lineAt(t).to,r=Math.max(n.from,e-Ai),o=Math.min(s,t+Ai),a=i.state.sliceDoc(r,o);if(r!=n.from){for(let l=0;l<Ai;l++)if(!Ti.test(a[l+1])&&Ti.test(a[l])){a=a.slice(l);break}}if(o!=s){for(let l=a.length-1;l>a.length-Ai;l--)if(!Ti.test(a[l-1])&&Ti.test(a[l])){a=a.slice(0,l);break}}return R.announce.of(`${i.state.phrase("current match")}. ${a} ${i.state.phrase("on line")} ${n.number}.`)}const j0=R.baseTheme({".cm-panel.cm-search":{padding:"2px 6px 4px",position:"relative","& [name=close]":{position:"absolute",top:"0",right:"4px",backgroundColor:"inherit",border:"none",font:"inherit",padding:0,margin:0},"& input, & button, & label":{margin:".2em .6em .2em 0"},"& input[type=checkbox]":{marginRight:".2em"},"& label":{fontSize:"80%",whiteSpace:"pre"}},"&light .cm-searchMatch":{backgroundColor:"#ffff0054"},"&dark .cm-searchMatch":{backgroundColor:"#00ffff8a"},"&light .cm-searchMatch-selected":{backgroundColor:"#ff6a0054"},"&dark .cm-searchMatch-selected":{backgroundColor:"#ff00ff8a"}}),U0=[Tt,pn.low(W0),j0],Ms=D.define({combine:i=>i[0]??""});function as(i,e){const t=Math.min(Math.max(e.line+1,1),i.lines),n=i.line(t);return Math.min(n.from+Math.max(e.character,0),n.to)}function Do(i,e){const t=i.lineAt(e);return{line:t.number-1,character:e-t.from}}const sd=N.define(),$0=me.define({create:()=>_.none,update(i,e){for(const t of e.effects)if(t.is(sd))return t.value;return i.map(e.changes)},provide:i=>R.decorations.from(i)}),sr={},K0=i=>sr[i]??(sr[i]=_.mark({class:`eye-${i}`}));function G0(i){const e=ce.fromClass(class{constructor(t){this.view=t,this.schedule(0)}update(t){t.docChanged&&this.schedule(40)}schedule(t){clearTimeout(this.timer),this.timer=setTimeout(()=>this.run(),t)}async run(){const t=this.view.state.doc;let n;try{n=await i.highlight(t.toString())}catch{return}if(this.view.state.doc!==t)return;const s=new At;for(const r of n)r.to>r.from&&r.to<=t.length&&s.add(r.from,r.to,K0(r.class));this.view.dispatch({effects:sd.of(s.finish())})}destroy(){clearTimeout(this.timer)}});return[$0,e]}const Nn=i=>i.replace(/[&<>"]/g,e=>({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;"})[e]);function Y0(i){return Nn(i).replace(/`([^`]+)`/g,"<code>$1</code>").replace(/\*\*([^*]+)\*\*/g,"<strong>$1</strong>").replace(/\[([^\]]+)\]\((https?:[^)\s]+)\)/g,'<a href="$2" target="_blank" rel="noopener">$1</a>').replace(/\[([^\]]+)\]\([^)]*\)/g,"$1")}async function rd(i,e){const t=document.createElement("div");t.className="eye-doc";const n=e.split(/```(\w*)\n([\s\S]*?)```/g);for(let s=0;s<n.length;s+=3){for(const r of n[s].split(/\n\s*\n/)){if(!r.trim())continue;const o=document.createElement("p");o.innerHTML=Y0(r.trim()),t.append(o)}if(s+2<n.length){const r=n[s+2].replace(/\n$/,""),o=document.createElement("pre");o.innerHTML=n[s+1]==="eye"?await J0(i,r):Nn(r),t.append(o)}}return t}async function J0(i,e){let t=[];try{t=await i.highlight(e)}catch{}let n="",s=0;for(const r of t)n+=Nn(e.slice(s,r.from))+`<span class="eye-${r.class}">${Nn(e.slice(r.from,r.to))}</span>`,s=r.to;return n+Nn(e.slice(s))}function X0(i){return Bh(async(e,t)=>{const n=e.state.facet(Ms);if(!n.endsWith(".eye"))return null;const s=e.state.wordAt(t);let r;try{r=await i.hover(n,Do(e.state.doc,t))}catch{return null}if(!r)return null;const o=await rd(i,r);return{pos:(s==null?void 0:s.from)??t,end:(s==null?void 0:s.to)??t,above:!0,create:()=>({dom:o})}})}const Q0={keyword:"keyword",function:"function",method:"method",field:"property",variable:"variable",type:"type",trait:"interface",module:"namespace",enum:"enum",variant:"enum",event:"constant",text:"text"};function Z0(i){return Wm({override:[async t=>{const n=t.state.facet(Ms);if(!n.endsWith(".eye"))return null;const s=t.matchBefore(/[A-Za-z_][A-Za-z0-9_]*/),r=t.matchBefore(/\.\s*$/)!==null||s!==null&&t.state.sliceDoc(s.from-1,s.from)===".";if(!s&&!r&&!t.explicit)return null;let o;try{o=await i.complete(n,Do(t.state.doc,t.pos))}catch{return null}return t.aborted||!o.length?null:{from:(s==null?void 0:s.from)??t.pos,validFor:/^[A-Za-z0-9_]*$/,options:o.map(a=>({label:a.label,type:Q0[a.kind]??"text",detail:a.detail,info:a.doc?()=>rd(i,a.doc):void 0}))}}],activateOnTyping:!0})}function ey(i,e){return a0(async t=>{const n=t.state.facet(Ms);let s;try{s=await i.check()}catch{return[]}e(s);const r=t.state.doc;return s.filter(o=>o.path===n).map(o=>{const a=as(r,o.range.start),l=Math.max(a,as(r,o.range.end));return{from:a,to:l,severity:"error",message:o.message,source:"eye"}})},{delay:350})}const ty=R.theme({"&":{height:"100%",fontSize:"14px",backgroundColor:"var(--vp-c-bg)",color:"var(--vp-c-text-1)"},".cm-scroller":{fontFamily:"var(--vp-font-family-mono)",lineHeight:"1.6"},".cm-gutters":{backgroundColor:"var(--vp-c-bg-soft)",color:"var(--vp-c-text-3)",border:"none"},".cm-activeLine":{backgroundColor:"var(--vp-c-default-soft)"},".cm-activeLineGutter":{backgroundColor:"var(--vp-c-default-soft)"},"&.cm-focused":{outline:"none"},".cm-content":{caretColor:"var(--vp-c-brand-1)"},".cm-cursor":{borderLeftColor:"var(--vp-c-brand-1)"},".cm-tooltip":{backgroundColor:"var(--vp-c-bg-elv)",border:"1px solid var(--vp-c-divider)",borderRadius:"6px",boxShadow:"var(--vp-shadow-3)"},".cm-tooltip-hover":{maxWidth:"560px",maxHeight:"320px",overflow:"auto",padding:"4px 10px"},".cm-selectionBackground, &.cm-focused .cm-selectionBackground, ::selection":{backgroundColor:"var(--vp-c-brand-soft) !important"}});function ny(i,e,t,n){const s=e.endsWith(".eye"),r=async(o,a)=>{if(!s)return!1;const l=await i.definition(e,Do(o.state.doc,a)).catch(()=>[]);return l.length&&n.onJump(l[0]),l.length>0};return z.create({doc:t,extensions:[Ms.of(e),hp(),up(),Ym(),Bf(),z.allowMultipleSelections.of(!0),z.tabSize.of(2),bs.of("  "),Kp(),Mm(),qf(),k0(),ty,co.of([{key:"Mod-Enter",run:()=>(n.onRun(),!0)},{key:"Mod-Shift-Enter",run:()=>(n.onTest(),!0)},{key:"Shift-Alt-f",run:()=>(n.onFormat(),!0)},{key:"Mod-s",run:()=>(n.onFormat(),!0),preventDefault:!0},{key:"F12",run:o=>(r(o,o.state.selection.main.head),!0)},...Rm,...Xg,...V0,...sg,...sc,...s0,Qg]),R.updateListener.of(o=>{o.docChanged&&n.onChange(e,o.state.doc.toString())}),R.domEventHandlers({mousedown(o,a){if(!(o.metaKey||o.ctrlKey)||o.button!==0)return!1;const l=a.posAtCoords({x:o.clientX,y:o.clientY});return l==null?!1:(o.preventDefault(),r(a,l),!0)}}),...s?[G0(i),X0(i),Z0(i),ey(i,n.onProblems),g0()]:[]]})}class dl{constructor(e){this.pending=new Map,this.next=1,this.synced=-1,this.status={state:"loading"},this.worker=new Worker(new URL("/assets/worker-D7gn_GOX.js",import.meta.url),{type:"module"}),this.worker.onmessage=t=>this.receive(t.data),this.worker.onerror=t=>this.setStatus({state:"failed",error:t.message||"the engine failed to start"}),this.worker.postMessage({init:{base:e}})}setStatus(e){var t;this.status=e,(t=this.onStatus)==null||t.call(this,e)}receive(e){var n;if(e.progress)return this.setStatus({state:"loading",...e.progress});if(e.ready)return this.setStatus({state:"ready"});if(e.failed)return this.setStatus({state:"failed",error:e.failed});if(e.exited)return this.setStatus({state:"failed",error:"the engine stopped; reload the page"});const t=this.pending.get(e.id);if(t){if(e.out)return(n=t.out)==null?void 0:n.call(t,e.out.stream,e.out.text);this.pending.delete(e.id),"error"in e?t.reject(new Error(e.error)):t.resolve(e.result)}}call(e,t,n){const s=this.next++;return new Promise((r,o)=>{this.pending.set(s,{resolve:r,reject:o,out:n}),this.worker.postMessage({id:s,op:e,args:t})})}sync(e,t){if(this.synced!==t)return this.synced=t,this.call("setFiles",{files:e})}terminate(e){this.worker.terminate();for(const t of this.pending.values())t.reject(new Error(e));this.pending.clear()}}class iy{constructor(e){this.files={},this.version=0,this.check=()=>this.tool("check"),this.hover=(t,n)=>this.tool("hover",{path:t,at:n}),this.complete=(t,n)=>this.tool("complete",{path:t,at:n}),this.definition=(t,n)=>this.tool("definition",{path:t,at:n}),this.format=t=>this.tool("format",{path:t}),this.highlight=t=>this.tools.call("highlight",{text:t}),this.base=e,this.tools=new dl(e),this.tools.onStatus=t=>{var n;return(n=this.onStatus)==null?void 0:n.call(this,t)}}get status(){return this.tools.status}setFiles(e){this.files={...e},this.version++}async tool(e,t){return await this.tools.sync(this.files,this.version),this.tools.call(e,t)}runnerWorker(){return this.runner||(this.runner=new dl(this.base)),this.runner}async run(e,t,n=1e4){const s=Intl.DateTimeFormat().resolvedOptions().timeZone;return this.limited(n,r=>r.call("run",{args:e,zone:s},t))}test(e,t=1e4){return this.limited(t,n=>n.call("test",{filter:e}))}async limited(e,t){const n=this.runnerWorker();await n.sync(this.files,this.version);let s;const r=new Promise((o,a)=>{s=setTimeout(()=>{this.stop(`stopped after ${e/1e3} s: the program may never end`),a(new Error(`stopped after ${e/1e3} s: the program may never end`))},e)});try{return await Promise.race([t(n),r])}finally{clearTimeout(s)}}stop(e="stopped"){var t;(t=this.runner)==null||t.terminate(e),this.runner=void 0}dispose(){this.stop(),this.tools.terminate("closed")}}const sy=`# The team's chat. Posting keeps what each key did, so posting again with the
# same key is how a lost reply is settled.

## What the chat did with a message.
pub type Post =
  | Posted(id: String)
  ## The channel is archived, or the bot was removed from it.
  | Refused(error: String)

# region contract
contract Chat {
  ## Posts a message to a channel. Each key is posted at most once.
  @key(required)
  @reconcile(post)
  command post(channel: String, text: String) -> Post
}
# endregion contract
`,ry=`# Marks the root of an Eyelang program: imports name directories below this one.
`,oy=`# A morning report. At 08:00 New York time on weekdays, the warehouse team
# hears how many paid orders wait to ship, and which has waited longest.
# The schedule carries its zone, so the report stays at 08:00 when the clocks
# change, and the key on the post means a restart never posts it twice.

import chat

let newYork: Zone = "America/New_York"

# region orders
entity Order(id: Int) {
  state cents: Int = 0
  state paidAt: Option(Instant) = None
  state shipped: Bool = false

  pub fn pay(amount: Int, at: Instant) {
    cents = amount
    paidAt = Some(at)
  }

  pub fn ship() { shipped = true }
}

## Every order that is paid for and not yet shipped. The runtime keeps it up
## to date as orders change, so the report reads it rather than every order.
view waiting = Order.all |> filter(fn o: o.paidAt != None && !o.shipped)
# endregion orders

# region summary
## An order that waits to ship, as the report sees it.
pub type Waiting = { id: Int, cents: Int, since: Instant }

## Writes cents as dollars: 41250 is "$412.50".
pub fn dollars(cents: Int) -> String {
  let rest = cents % 100
  "\${cents / 100}.{if rest < 10 then "0{rest}" else "{rest}"}"
}

## The report's text, or None when nothing waits. It is pure: the orders and
## the time come in as arguments, so a test can pass any.
pub fn summary(orders: List(Waiting), at: Instant) -> Option(String) {
  let oldest = orders |> map(fn o: (o.since, o.id)) |> sort() |> first()?
  let (since, id) = oldest
  let total = orders |> map(fn o: o.cents) |> sum()
  let count = if orders |> size == 1 then "1 paid order waits" else "{orders |> size} paid orders wait"
  Some("{count} to ship, {dollars(total)} in all. #{id} has waited longest: {at - since}.")
}
# endregion summary

# region report
## Posts the report on weekday mornings. The key names the day, so however
## often this step runs, the team sees one report a day.
@on(weekdays(08:00, in: newYork))
job report(day: Date) {
  let at = now!()
  let open = waiting.read!() |> map(fn o: { id: o.id, cents: o.cents, since: o.paidAt ?? at })
  let Some(text) = summary(open, at) else { return }
  match chat.post!("#warehouse", text, key: "report-{day}").reconcile!() {
    Posted(_) => ()
    Refused(_) => log.warn("the chat refused the report for {day}")
  }
}
# endregion report

# ---------------------------------------------------------------- tests

# region test-summary
test "the summary counts the orders and finds the oldest" {
  let at = 2026-10-30 12:00 in "UTC"
  let orders = [{ id: 2, cents: 38_750, since: at - 1 day }, { id: 1, cents: 2_500, since: at - 3 days }]
  expect(summary(orders, at) == Some("2 paid orders wait to ship, $412.50 in all. #1 has waited longest: 3 days."))
  expect(summary([], at) == None)
}
# endregion test-summary

# region test-schedule
test "the report comes at 08:00 in New York, on both sides of the clock change" {
  let team = fakes(chat, from: "recordings/")
  with team
  with clock(2026-10-30 11:00 in "UTC")  # Friday, 07:00 in New York
  Order(1182).pay(4_250, now() - 1 day)
  Order(1183).pay(1_999, now())
  Order(1183).ship()
  advance(1 h)  # 08:00 in New York is 12:00 UTC on summer time
  let [friday] = team.calls(chat.post) else { unreachable("no report on Friday") }
  expect(friday.text == "1 paid order waits to ship, $42.50 in all. #1182 has waited longest: 25 h.")
  # No report at the weekend. On Sunday the clocks go back an hour, so
  # Monday's 08:00 is 13:00 UTC.
  advance(3 days)
  expect(team.calls(chat.post) |> size == 1)
  advance(1 h)
  let [_, monday] = team.calls(chat.post) else { unreachable("no report on Monday") }
  expect(monday.key == "report-2026-11-02")
}
# endregion test-schedule

test "nothing waiting, nothing posted" {
  let team = fakes(chat, from: "recordings/")
  with team
  with clock(2026-10-30 11:00 in "UTC")
  advance(1 day)
  expect(team.calls(chat.post) == [])
}
`,ay=`{
  "contract": "Chat",
  "exchanges": [
    { "op": "post", "key": "report-2026-10-30", "result": { "kind": "Posted", "id": "msg-1" } },
    { "op": "post", "key": "report-2026-11-02", "result": { "kind": "Posted", "id": "msg-2" } }
  ]
}
`,ly=`---
title: Examples
---

# Examples

Each example below is a whole program for a problem most backend developers have met: a reminder
before a trial ends, webhooks to a partner who is sometimes down, an uptime check, a morning
report, a refund that needs a person's yes. Each shows a different thing Eyelang does for you, and
each has tests that pass. The tests run days of waiting in milliseconds, and they answer calls to
other services from recorded replies instead of the network.

Open any of them in the [playground](/playground) and press **Test** to run its tests in your
browser. The source of each is in
[docs/examples](https://github.com/ruxxzebre/eyelang/tree/dev/docs/examples).

A few words used below:

- An **entity** is a piece of state with one owner, such as one account. Its data changes only
  through its own methods, each a transaction.
- A **job** is a function that can wait for minutes or days. Each step marked \`!\` is saved, so a run
  that waits is kept on disk, not in memory, and carries on after a restart.
- A **view** is a list of entities kept up to date as they change. A job can start when an entity
  enters or leaves it.
- A **contract** describes another service's API: its types and operations. A **key** on a call
  lets the service see a repeat and act once.

## Trial ending: an email three days before

A sign-up gets a 14-day trial and one email three days before it ends, unless they upgrade first.
Nothing polls a \`trial_ends_at\` column: the job sleeps until the time comes. Because a trial can be
extended while the job sleeps, the job reads the account again each time it wakes. An upgrade ends
the run, and an extension sends it back to sleep.

<<< ./trial-ending/main.eye#remind

The tests sign up, move the clock with \`advance(5 days)\`, upgrade or extend, and check what was
sent. [Open it in the playground](/playground#example=examples/trial-ending).

## Webhook delivery: keys, lost replies and backoff

The shop tells a partner about each order event. Three things can go wrong, and each has its
answer. An event queued twice is delivered once, because the delivery job starts once per key. A
reply lost on the way back is \`Uncertain\`, and \`.reconcile!()\` asks the partner what happened
instead of sending again. A partner that is down is tried again on a schedule the program writes
for itself. Eyelang has no retry library: the policy is a list of waits and a short waiting
function, and each \`sleep!\` in it is a saved step.

<<< ./webhook-delivery/main.eye#retry

<<< ./webhook-delivery/main.eye#deliver

The partner's recordings list several replies for one key, which answer the tries in turn: three
503s and then an \`Ok\` check that the delivery gets through at 09:36 and not before, and a lost
reply, a receipt query that says it never arrived and an \`Ok\` check that \`.reconcile!()\` sends it
again with the same key.
[Open it in the playground](/playground#example=examples/webhook-delivery).

## Uptime monitor: probe every minute, alert once

Every minute each site gets an HTTP GET. Two failures in a row put the site in the view \`down\`.
The team hears once when a site enters the view and once when it leaves, so there is no
"already alerted" flag to keep in step. \`@concurrency(1)\` stops a slow round of probes from piling
up behind the next.

<<< ./uptime-monitor/main.eye#monitor

The tests fake \`http.get\` from recordings of a site that is up and one that answers 503, and check
that ten failed probes give one message.
[Open it in the playground](/playground#example=examples/uptime-monitor).

## Daily report: 08:00 in New York, every weekday

At 08:00 New York time on weekdays, the warehouse team hears how many paid orders wait to ship. The
schedule names its zone, so the report stays at 08:00 when the clocks change, with no second
schedule and no check for summer time. The post's key names the day, so a restart never posts the
report twice.

<<< ./daily-report/main.eye#report

The test runs from a Friday to the Monday after the clocks go back, and checks that Monday's report
comes at 13:00 UTC, an hour later than Friday's.
[Open it in the playground](/playground#example=examples/daily-report).

## Refund approval: a person says yes first

Support asks for a refund, finance has a day to answer, and then the head of finance has a day.
The payment provider's \`refund\` is \`@destructive\`, so it takes an \`Approval\`, and only
\`approval!(…)\` makes one. A refund without a person's yes doesn't compile. Only the person asked
can answer, and the run waits on disk for the two days, through restarts.

<<< ./refund-approval/main.eye#review

The tests answer as finance, as someone who wasn't asked, and not at all.
[Open it in the playground](/playground#example=examples/refund-approval).

## Log summary: the plain language

Not every program waits. This one reads a web server's access log and prints a table of requests,
failures and latency per path. It shows the core language on its own: records and variant types,
errors as values with \`Result\` and \`?\`, a generic \`percentile\` that works on anything that can be
ordered, a trait for table rows, and pipes. A line it can't read is reported with its line number,
and the rest are still counted.

<<< ./log-summary/main.eye#table

Press **Run** in the playground to see the table for a short sample log.
[Open it in the playground](/playground#example=examples/log-summary).

## A shop and its bank: two programs, one contract

The shop charges customers through the bank's \`Payments\` contract, and the bank, a second program
in \`bank/\`, serves it. In the playground, **Run** starts both and shows each call as it crosses
from one to the other. The same order charged twice is paid once, because the key names the order.

<<< ./shop-and-bank/main.eye#checkout

[Open it in the playground](/playground#example=examples/shop-and-bank).

## More

- The [tutorial](/tutorial/) builds a program step by step, from values to a job that waits.
- The [guide](/guide/) explains each part of the language with a program of its own.
`,hy=`# Marks the root of an Eyelang program: imports name directories below this one.
`,cy=`# Access log summary. Reads a web server's access log and prints a table:
# for each path, how many requests it had, how many failed, and how slow it
# was. Plain Eyelang, with no jobs or entities: records and variants, errors
# as values, a trait, a generic function and pipes.

import fs

# region parse
## One request, as one line of the log records it.
pub type Request = { method: String, path: String, status: Int, ms: Int }

## Why a line couldn't be read.
pub type LineError =
  | WrongFields(found: Int)
  | NotANumber(field: String, text: String)

## How a LineError is written in a message, in place of the derived
## \`NotANumber(field: "ms", text: "fast")\`.
impl Show for LineError {
  fn show(self) = match self {
    WrongFields(found) => "expected 5 fields, found {found}"
    NotANumber(field, text) => "the {field} should be a number, and is \\"{text}\\""
  }
}

## Reads a number, or says which field held something else.
fn number(field: String, text: String) -> Result(Int, LineError) =
  parseInt(text).mapErr(fn _: NotANumber(field:, text:))

## Reads one line: \`2026-10-12T09:00:01Z GET /api/orders 200 34\`, where the
## last two fields are the status and the time taken, in milliseconds.
pub fn parseLine(line: String) -> Result(Request, LineError) {
  let fields = line |> words()
  let [_, method, path, status, ms] = fields else { return Err(WrongFields(found: fields |> size)) }
  Ok({ method, path, status: number("status", status)?, ms: number("ms", ms)? })
}
# endregion parse

# region stats
## The value below which \`p\` percent of \`xs\` fall, for any type that can be
## ordered: \`percentile([3, 1, 2], 50)\` is \`Some(2)\`.
pub fn percentile(xs: List(a), p: Int) -> Option(a) where a: Ord {
  let sorted = xs |> sort()
  sorted[(sorted |> size) * p / 100]
}

## What one path's requests add up to. Records are ordered by their fields
## in the order written here, so sorting these puts the least busy first.
pub type PathStats = { requests: Int, path: String, failed: Int, p50: Int, p95: Int }

## Sums up one path's requests. A 5xx status counts as failed.
fn stats(path: String, rs: List(Request)) -> PathStats {
  let times = rs |> map(fn r: r.ms)
  {
    path,
    requests: rs |> size,
    failed: rs |> count(fn r: r.status >= 500),
    p50: percentile(times, 50) ?? 0,
    p95: percentile(times, 95) ?? 0,
  }
}

## Groups requests by path, then sums up each group, busiest path first.
pub fn byPath(requests: List(Request)) -> List(PathStats) {
  var groups: Map(String, List(Request)) = {}
  for r in requests {
    groups[r.path] = (groups[r.path] ?? []) + [r]
  }
  groups
    |> entries()
    |> map(fn (path, rs): stats(path, rs))
    |> sort()
    |> reverse()
}
# endregion stats

# region table
## Anything a report shows as one row of a table.
trait Row {
  fn cells(self) -> List(String)
}

impl Row for PathStats {
  fn cells(self) = [self.path, "{self.requests}", "{self.failed}", "{self.p50} ms", "{self.p95} ms"]
}

## Pads \`s\` with spaces to \`width\` characters.
fn pad(s: String, width: Int) -> String {
  let n = s.chars() |> toList() |> size
  s + ((0..max(width - n, 0)) |> map(fn _: " ") |> join(""))
}

## Lays out rows of any type that is a Row, under a header, in columns.
pub fn table(header: List(String), rows: List(r)) -> String where r: Row {
  let lines = [header] + (rows |> map(fn r: r.cells()))
  let widths = header
    |> enumerate()
    |> map(fn (i, _): lines |> map(fn l: (l[i] ?? "").chars() |> toList() |> size) |> reduce(0, max))
  lines
    |> map(fn l: l |> zip(widths) |> map(fn (cell, w): pad(cell, w)) |> join("  ") |> trim())
    |> join("\\n")
}
# endregion table

# region main
## A few lines of a log, summed up when no file is named.
let sample = """
  2026-10-12T09:00:01Z GET /api/orders 200 34
  2026-10-12T09:00:01Z GET /health 200 2
  2026-10-12T09:00:02Z POST /api/orders 201 120
  2026-10-12T09:00:03Z GET /api/orders 200 41
  2026-10-12T09:00:04Z POST /api/orders 503 2004
  2026-10-12T09:00:06Z GET /health 200 3
  2026-10-12T09:00:07Z GET /api/orders/1183 404 9
  2026-10-12T09:00:08Z the load balancer restarted
  2026-10-12T09:00:10Z GET /api/orders 500 15
  """

## Prints the summary of the log file named, or of the sample:
## \`eye run . -- /var/log/nginx/access.log\`.
pub fn main(args: List(String)) -> Result((), Problem) uses {fs, stdio} {
  let path = args[1] ?? "sample"
  let text = if path == "sample" then sample else fs.readText(path).context("reading {path}")?
  var requests = []
  for (n, line) in text |> lines() |> enumerate() {
    match parseLine(line) {
      Ok(r) => requests += [r]
      Err(e) => log.warn("{path}:{n + 1}: {e}")
    }
  }
  print(table(["path", "requests", "failed", "p50", "p95"], byPath(requests)))
  Ok(())
}
# endregion main

# ---------------------------------------------------------------- tests

# region tests
test "a line is read into a request, or says what is wrong with it" {
  expect(parseLine("2026-10-12T09:00:01Z GET /api/orders 200 34") == Ok({ method: "GET", path: "/api/orders", status: 200, ms: 34 }))
  expect(parseLine("GET /api/orders 200") == Err(WrongFields(found: 3)))
  expect(parseLine("2026-10-12T09:00:01Z GET / 200 fast") == Err(NotANumber(field: "ms", text: "fast")))
}

test "percentile works on anything that can be ordered" {
  expect(percentile([30, 10, 20, 40], 50) == Some(30))
  expect(percentile(["b", "a", "c"], 0) == Some("a"))
  let none: List(Int) = []
  expect(percentile(none, 95) == None)
}

test "requests are summed up per path, busiest first" {
  let log: List(Request) = [
    { method: "GET", path: "/health", status: 200, ms: 5 },
    { method: "GET", path: "/api/orders", status: 200, ms: 30 },
    { method: "POST", path: "/api/orders", status: 503, ms: 900 },
  ]
  let stats = byPath(log)
  expect(stats |> map(fn s: s.path) == ["/api/orders", "/health"])
  expect(table(["path", "requests", "failed"], stats |> take(1)) == "path         requests  failed\\n/api/orders  2         1")
}
# endregion tests
`,dy=`# Marks the root of an Eyelang program: imports name directories below this one.
`,uy=`# The mail service. This contract is its interface; the refunds program calls
# it.

## What the service did with a message.
pub type Delivery =
  ## The service has it. A repeated key gives the first answer again.
  | Sent(id: String)
  ## An address the service can't deliver to.
  | Bounced(reason: String)

# region contract
contract Mail {
  ## Sends one message. Each key is sent at most once.
  @key(required)
  @reconcile(sent)
  command send(to: String, subject: String, text: String) -> Delivery

  ## What happened to the message sent with this key, if it arrived.
  query sent(key: Key) -> Option(Delivery)
}
# endregion contract
`,fy=`# Refund approvals. Support asks for a refund; finance says yes or no within
# a day; if nobody in finance answers, the head of finance is asked; then the
# money goes back and the customer hears what happened. The payment
# provider's refund needs an Approval, so the compiler rejects any path that
# skips the person.

import mail
import payments

# region request
pub type Request = { order: Int, cents: Int, customer: String, reason: String }

pub type Status =
  | New
  | Waiting
  | Done(refund: String, approvedBy: String)
  | Refused(why: String)
  ## Nobody answered within two days.
  | Unanswered

entity RefundRequest(id: String) {
  state status: Status = New

  ## Takes a request in and starts its review, in one transaction. Asking
  ## again with the same id starts nothing.
  pub fn submit(r: Request) uses {start review} {
    if status != New { return }
    status = Waiting
    review.start(id, r, key: "review-{id}")
  }

  pub fn close(s: Status) { status = s }
}
# endregion request

## Writes cents as dollars: 25000 is "$250.00".
pub fn dollars(cents: Int) -> String {
  let rest = cents % 100
  "\${cents / 100}.{if rest < 10 then "0{rest}" else "{rest}"}"
}

# region review
## Asks finance, and the head of finance if finance doesn't answer within a
## day. Each wait is a durable step: the run sleeps on disk for as long as it
## takes, through restarts, and holds no memory while it does.
fn decide(what: String) -> Decision {
  match approval!(what, from: "finance", within: 1 day) {
    Expired => approval!("{what} (finance didn't answer)", from: "head-of-finance", within: 1 day)
    answered => answered
  }
}

## Reviews one request, refunds it if a person approved, and tells the
## customer.
job review(id: String, r: Request) {
  let status = match decide!("Refund {dollars(r.cents)} on order #{r.order}: {r.reason}") {
    Approved(ok) => refund!(id, r, ok)
    Declined => Refused(why: "declined")
    Expired => Unanswered
  }
  RefundRequest(id).close!(status)
  tell!(id, r, status)
}

## Gives the money back. payments.refund takes the Approval that decide! got;
## without one, this doesn't compile.
fn refund(id: String, r: Request, ok: Approval) -> Status {
  match payments.refund!(r.order, r.cents, approval: ok, key: "refund-{id}").reconcile!() {
    Refunded(refund) => Done(refund:, approvedBy: ok.by)
    NotRefundable(reason) => Refused(why: reason)
  }
}
# endregion review

## Tells the customer how their refund went. The key is the request's, so
## they hear once. An unanswered request is support's to follow up, so the
## customer isn't told it was refused.
fn tell(id: String, r: Request, s: Status) {
  let text = match s {
    Done(..) => "We have refunded {dollars(r.cents)} for order #{r.order}."
    Refused(..) => "We can't refund order #{r.order}. Reply to this email to talk to us."
    New | Waiting | Unanswered => return
  }
  match mail.send!(to: r.customer, subject: "Your refund", text:, key: "refund-{id}").reconcile!() {
    Sent(_) => ()
    Bounced(reason) => log.warn("the refund email for {id} bounced: {reason}")
  }
}

# ---------------------------------------------------------------- tests

let broken: Request = { order: 1182, cents: 25_000, customer: "ann@example.com", reason: "arrived broken" }

# region tests
test "finance approves, and the money goes back once" {
  let payer = fakes(payments, from: "recordings/")
  with payer
  with fakes(mail, from: "recordings/")
  RefundRequest("r-1").submit(broken)
  RefundRequest("r-1").submit(broken)
  approve("Refund $250.00 on order #1182")
  expect(RefundRequest("r-1").status == Done(refund: "re-1", approvedBy: "finance"))
  expect(payer.calls(payments.refund) |> size == 1)
}

test "only the person asked can approve" {
  with fakes(payments, mail, from: "recordings/")
  RefundRequest("r-2").submit(broken)
  approve("Refund $250.00", by: "support")
  expect(RefundRequest("r-2").status == Waiting)
  decline("Refund $250.00")
  expect(RefundRequest("r-2").status == Refused(why: "declined"))
}

test "when finance doesn't answer within a day, the head of finance is asked" {
  with fakes(payments, mail, from: "recordings/")
  RefundRequest("r-3").submit(broken)
  advance(1 day)
  approve("finance didn't answer")
  expect(RefundRequest("r-3").status == Done(refund: "re-3", approvedBy: "head-of-finance"))
}

test "when nobody answers, nothing is refunded and support follows up" {
  let payer = fakes(payments, from: "recordings/")
  let sent = fakes(mail, from: "recordings/")
  with payer
  with sent
  RefundRequest("r-4").submit(broken)
  advance(2 days)
  expect(RefundRequest("r-4").status == Unanswered)
  expect(payer.calls(payments.refund) == [] && sent.calls(mail.send) == [])
}
# endregion tests

test "a refund the provider refuses is told to the customer" {
  with fakes(payments, mail, from: "recordings/")
  RefundRequest("r-5").submit(broken)
  approve("Refund $250.00")
  expect(RefundRequest("r-5").status == Refused(why: "the charge is more than 180 days old"))
}
`,py=`# The payment provider. This contract is the part of its API the refunds
# program calls.

## What the provider did with a refund.
pub type Refund =
  ## The money is on its way back to the customer.
  | Refunded(id: String)
  ## The provider can't refund this order, for example because its charge is
  ## too old.
  | NotRefundable(reason: String)

# region contract
contract Payments {
  ## Gives money back for an order. @destructive adds a parameter \`approval\`,
  ## and only approval!(…) makes an Approval, so no code can call this
  ## without a person having said yes.
  @destructive
  @key(required)
  @reconcile(refundFor)
  command refund(order: Int, cents: Int) -> Refund

  ## What the refund sent with this key did, if it arrived.
  query refundFor(key: Key) -> Option(Refund)
}
# endregion contract
`,my=`{
  "contract": "Mail",
  "exchanges": [
    { "op": "send", "key": "refund-r-1", "result": { "kind": "Sent", "id": "m-1" } },
    { "op": "send", "key": "refund-r-2", "result": { "kind": "Sent", "id": "m-2" } },
    { "op": "send", "key": "refund-r-3", "result": { "kind": "Sent", "id": "m-3" } },
    { "op": "send", "key": "refund-r-5", "result": { "kind": "Bounced", "reason": "no such mailbox" } }
  ]
}
`,gy=`{
  "contract": "Payments",
  "exchanges": [
    { "op": "refund", "key": "refund-r-1", "result": { "kind": "Refunded", "id": "re-1" } },
    { "op": "refund", "key": "refund-r-3", "result": { "kind": "Refunded", "id": "re-3" } },
    {
      "op": "refund",
      "key": "refund-r-5",
      "result": { "kind": "NotRefundable", "reason": "the charge is more than 180 days old" }
    }
  ]
}
`,yy=`# The bank. It serves Payments: each operation is one function, run as one
# transaction. In the playground the shop's calls reach it in the page, with
# no network, as two \`eye run\`s would on real ports.

import payments.{Charge, Charged, CustomerId, Declined}

entity Account(id: CustomerId) {
  state balanceCents: Int = 10_000

  pub fn take(cents: Int) -> Charge {
    if cents > balanceCents { return Declined(reason: "only {balanceCents} cents left") }
    balanceCents -= cents
    Charged(balanceCents:)
  }
}

## What each charge did, by its key, so a charge sent again is answered from
## here and takes nothing.
entity Ledger(key: Key) {
  state charge: Option(Charge) = None

  pub fn record(c: Charge) { charge = Some(c) }
}

impl payments.Payments {
  fn charge(customer, amountCents, key) {
    match Ledger(key).charge {
      Some(done) => done
      None => {
        let c = Account(customer).take(amountCents)
        Ledger(key).record(c)
        match c {
          # Sent to the shop when this transaction commits.
          Charged(_) => payments.charged.publish({ charge: "{key}", customer, amountCents })
          Declined(_) => ()
        }
        c
      }
    }
  }

  fn chargeFor(key) = Ledger(key).charge

  fn balance(customer) = Account(customer).balanceCents
}

test "a charge sent twice takes the money once" {
  let ann = CustomerId("ann")
  expect(Account(ann).take(2_500) == Charged(balanceCents: 7_500))
  expect(Account(ann).take(9_000) == Declined(reason: "only 7500 cents left"))
}
`,by=`# The bank: a program of its own that serves Payments. It finds the contract's
# module in the directory above.
modules = [".."]
`,wy=`# The shop: the program the playground runs. bank/ has an eye.toml of its own,
# so it is a second program, which serves the Payments contract.
`,vy=`# A shop that charges its customers through the bank's Payments contract. Run
# it: the bank, the program in bank/, answers each call in the page, and the
# output shows every call as it crosses. Test it: the tests answer the calls
# from recordings instead.

import payments.{Charged, CustomerId, Declined}

entity Order(id: String) {
  state status: String = "open"

  pub fn settle(s: String) { status = s }
}

# region checkout
## Charges a customer for an order, once however often it runs: the key
## names the order.
job checkout(customer: CustomerId, order: String, cents: Int) uses {entity Order, net Payments.charge, stdio, wait} {
  let charge = payments.charge!(customer, cents, key: "order-{order}").reconcile!()
  let status = match charge {
    Charged(left) => "paid {cents} cents, {left} left"
    Declined(reason) => "declined: {reason}"
  }
  Order(order).settle!(status)
  print("order {order}: {status}")
}
# endregion checkout

## Runs for each charge the bank tells the shop about: its event crosses
## between the programs as a POST, as it would between two \`eye run\`s.
@on(event(payments.charged))
job receipt(r: payments.Receipt) {
  print("receipt for {r.charge}: {r.amountCents} cents")
}

## Says what a customer has left.
job report(customer: CustomerId) uses {net Payments.balance, stdio, wait} {
  print("{customer} has {payments.balance!(customer)} cents")
}

pub fn main() uses {start checkout, start report} {
  let ann = CustomerId("ann")
  checkout.start(ann, "1", 2_500, key: "checkout-1")
  # The same order again: the bank answers from its ledger.
  checkout.start(ann, "1", 2_500, key: "checkout-1-again")
  checkout.start(ann, "2", 9_000, key: "checkout-2")
  report.start(ann, key: "report")
}

test "an order is paid once, and a declined one says why" {
  with fakes(payments, from: "recordings/")
  checkout.start(CustomerId("ann"), "1", 2_500, key: "t1")
  checkout.start(CustomerId("ann"), "2", 9_000, key: "t2")
  expect(Order("1").status == "paid 2500 cents, 7500 left")
  expect(Order("2").status == "declined: only 7500 cents left")
}
`,ky=`# The contract between the shop and the bank: what the bank serves and the shop
# calls. Both programs import this module.

pub type CustomerId = | CustomerId(String)

## What the bank did with a charge.
pub type Charge =
  ## The money was taken.
  | Charged(balanceCents: Int)
  ## Nothing was taken.
  | Declined(reason: String)

## A charge that took money, which the bank tells its subscribers.
pub type Receipt = { charge: String, customer: CustomerId, amountCents: Int }

@port(8090)
@http(names: snake)
@auth(bearer: "payments-token")
contract Payments {
  ## Takes money from a customer's account. The key names the charge, so a
  ## charge sent twice takes the money once.
  @http("POST /v1/charges", status: { 201: Charged, 402: Declined })
  @key(required)
  @reconcile(chargeFor)
  command charge(customer: CustomerId, amountCents: Int) -> Charge

  ## What the charge with this key did, or None if it never arrived.
  @http("GET /v1/charges/by-key/:key")
  query chargeFor(key: Key) -> Option(Charge)

  ## A customer's balance in cents.
  @http("GET /v1/customers/:customer/balance")
  query balance(customer: CustomerId) -> Int

  ## Published when a charge takes money. A program that waits for it gets
  ## it as \`POST /events/payments.charged\`; the key makes a repeat start
  ## nothing.
  @key(charge)
  event charged: Receipt on "payments.charged"
}
`,xy=`{
  "contract": "Payments",
  "exchanges": [
    {
      "op": "charge",
      "key": "order-1",
      "result": { "kind": "Charged", "balanceCents": 7500 }
    },
    {
      "op": "charge",
      "key": "order-2",
      "result": { "kind": "Declined", "reason": "only 7500 cents left" }
    }
  ]
}
`,Sy=`# Marks the root of an Eyelang program: imports name directories below this one.
`,Cy=`# The mail service. This contract is its interface; the trial reminders call
# it.

## What the service did with a message.
pub type Delivery =
  ## The service has it. A repeated key gives the first answer again.
  | Sent(id: String)
  ## An address the service can't deliver to.
  | Bounced(reason: String)

# region contract
contract Mail {
  ## Sends one message. Each key is sent at most once.
  @key(required)
  @reconcile(sent)
  command send(to: String, subject: String, text: String) -> Delivery

  ## What happened to the message sent with this key, if it arrived.
  query sent(key: Key) -> Option(Delivery)
}
# endregion contract
`,Ay=`# Trial reminders. A sign-up gets a 14-day trial, and one email three days
# before it ends, unless they have upgraded by then. Extending a trial moves
# the email; nothing polls a table to find it.

import mail

## How long a trial lasts.
let trialLength = 14 days

## How long before the end of a trial its reminder goes.
let notice = 3 days

# region account
pub type Plan =
  | NotSignedUp
  | Trial(ends: Instant)
  | Paid

entity Account(id: String) {
  state email: String = ""
  state plan: Plan = NotSignedUp
  state bounced: Option(String) = None

  ## Starts the trial and its reminder, in one transaction. Signing up again
  ## changes nothing and starts nothing.
  pub fn signUp(address: String, at: Instant) uses {start remind} {
    if plan != NotSignedUp { return }
    email = address
    plan = Trial(ends: at + trialLength)
    remind.start(id, key: "trial-{id}")
  }

  pub fn upgrade() { plan = Paid }

  pub fn extend(by: Duration) {
    let Trial(ends) = plan else { return }
    plan = Trial(ends: ends + by)
  }

  pub fn bounce(reason: String) { bounced = Some(reason) }
}
# endregion account

# region remind
## Sleeps until three days before the trial ends, then emails the customer.
## The trial can change while the run sleeps, so it reads the account again
## each time it wakes: an upgrade ends the run, and an extension sends it
## back to sleep.
job remind(id: String) {
  while true {
    let account = Account(id).read!()
    let Trial(ends) = account.plan else { return }
    if now!() >= ends - notice {
      email!(id, account.email)
      return
    }
    sleepUntil!(ends - notice)
  }
}
# endregion remind

## Sends the reminder. The key is the account's, so it goes once.
fn email(id: String, to: String) {
  let text = "Your trial ends in 3 days. Upgrade to keep your projects: https://example.com/upgrade"
  match mail.send!(to:, subject: "Your trial ends soon", text:, key: "trial-{id}").reconcile!() {
    Sent(_) => ()
    Bounced(reason) => Account(id).bounce!(reason)
  }
}

## Signs one customer up from the command line, for example
## \`eye run . -- ann ann@example.com\`.
pub fn main(args: List(String)) -> Result((), String) uses {clock, entity Account, start remind, stdio} {
  let (Some(id), Some(email)) = (args[1], args[2]) else {
    return Err("usage: eye run . -- ID EMAIL")
  }
  Account(id).signUp(email, now())
  print("{id}'s trial ends at {now() + trialLength}; the reminder goes 3 days before")
  Ok(())
}

# ---------------------------------------------------------------- tests

# region tests
test "a trial gets one reminder, three days before it ends" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  with clock(2026-10-01 09:00 in "UTC")
  Account("ann").signUp("ann@example.com", now())
  advance(10 days + 23 h)
  expect(sent.calls(mail.send) == [])
  advance(1 h)  # 2026-10-12 09:00: the trial ends on the 15th
  let [reminder] = sent.calls(mail.send) else { unreachable("not one reminder") }
  expect(reminder.to == "ann@example.com")
  advance(30 days)
  expect(sent.calls(mail.send) |> size == 1)
}

test "an upgrade means no reminder" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  Account("ann").signUp("ann@example.com", now())
  advance(5 days)
  Account("ann").upgrade()
  advance(30 days)
  expect(sent.calls(mail.send) == [])
}

test "an extension moves the reminder" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  Account("ann").signUp("ann@example.com", now())
  advance(5 days)
  Account("ann").extend(7 days)
  advance(12 days)  # day 17: the old reminder's day, 11, has passed
  expect(sent.calls(mail.send) == [])
  advance(1 day)  # day 18: three days before the new end, day 21
  expect(sent.calls(mail.send) |> size == 1)
}
# endregion tests

test "a reminder that bounces is kept on the account" {
  with fakes(mail, from: "recordings/")
  Account("bo").signUp("bo@example.com", now())
  advance(11 days)
  expect(Account("bo").bounced == Some("no such mailbox"))
}
`,Ty=`{
  "contract": "Mail",
  "exchanges": [
    { "op": "send", "key": "trial-ann", "result": { "kind": "Sent", "id": "m-1" } },
    { "op": "send", "key": "trial-bo", "result": { "kind": "Bounced", "reason": "no such mailbox" } }
  ]
}
`,My=`# The team's chat. Posting keeps what each key did, so posting again with the
# same key is how a lost reply is settled.

## What the chat did with a message.
pub type Post =
  | Posted(id: String)
  ## The channel is archived, or the bot was removed from it.
  | Refused(error: String)

# region contract
contract Chat {
  ## Posts a message to a channel. Each key is posted at most once.
  @key(required)
  @reconcile(post)
  command post(channel: String, text: String) -> Post
}
# endregion contract
`,Oy=`# Marks the root of an Eyelang program: imports name directories below this one.
`,Iy=`# Uptime monitor. Every minute each watched site gets an HTTP GET. Two failed
# probes in a row mean the site is down. The team hears once when it goes
# down and once when it is back, with how long it was out.

import chat
import http

## Where alerts go.
let channel = "#ops"

# region probe
## What one probe found.
pub type Probe =
  | Up
  | Down(why: String)

## Reads a probe's answer. A 5xx, or no answer at all, is down.
pub fn read(answer: Result(http.Response, http.HttpError)) -> Probe = match answer {
  Ok(page) => if page.status < 500 then Up else Down(why: "it answered {page.status}")
  Err(error) => Down(why: "{error}")
}
# endregion probe

# region site
## A site to watch, and what its recent probes found.
entity Site(name: String) {
  state url: String = ""
  ## Failed probes in a row.
  state failures: Int = 0
  state why: String = ""
  state failingSince: Option(Instant) = None
  state outages: Int = 0
  state lastOutage: Duration = 0 s

  pub fn watch(address: String) { url = address }

  ## Keeps what one probe found.
  pub fn record(p: Probe, at: Instant) {
    match p {
      Down(reason) => {
        if failures == 0 { failingSince = Some(at) }
        failures += 1
        why = reason
        if failures == 2 { outages += 1 }
      }
      Up => {
        if failures >= 2 { lastOutage = at - (failingSince ?? at) }
        failures = 0
        failingSince = None
      }
    }
  }
}
# endregion site

# region monitor
## Probes every site once a minute. With @concurrency(1), a round that runs
## long makes the next minute's round skip, rather than pile up behind it.
@on(every(1 min))
@concurrency(1)
job probe() {
  for site in Site.all.read!() {
    let found = read(http.get!(site.url))
    Site(site.name).record!(found, now!())
  }
}

## Every site that failed two probes in a row and hasn't answered since.
view down = Site.all |> filter(fn s: s.failures >= 2)

## A site that enters the view has just gone down. The view says when, so
## there is no "alerted" flag to keep: the run starts once per outage.
@on(enters(down))
job wentDown(s: Ref(Site)) {
  let site = s.read!()
  tell!("{site.name} is down: {site.why}", key: "down-{site.name}-{site.outages}")
}

## A site that leaves the view is back.
@on(leaves(down))
job cameBack(s: Ref(Site)) {
  let site = s.read!()
  tell!("{site.name} is back after {site.lastOutage}", key: "up-{site.name}-{site.outages}")
}
# endregion monitor

@unwitnessed("the chat refuses only an archived channel, which no recording has")
fn tell(text: String, key: Key) {
  match chat.post!(channel, text, key:).reconcile!() {
    Posted(_) => ()
    Refused(error) => log.warn("the chat refused {key}: {error}")
  }
}

## Watches the sites named on the command line, for example
## \`eye run . -- shop=https://shop.example.com api=https://api.example.com\`.
pub fn main(args: List(String)) uses {entity Site} {
  for arg in args |> drop(1) {
    let [name, url] = arg |> split("=") else { continue }
    Site(name).watch(url)
  }
}

# ---------------------------------------------------------------- tests

fn posted(f: Fakes) -> List(String) = f.calls(chat.post) |> map(fn c: c.text)

# region test-outage
test "an outage is told once when it starts, and once when it ends" {
  let team = fakes(chat, from: "recordings/")
  with team
  with clock(2026-10-12 09:00 in "UTC")
  with fakes(http, from: "recordings/up/")
  Site("shop").watch("https://shop.example.com")
  advance(5 min)
  with fakes(http, from: "recordings/down/")
  advance(1 min)  # one failed probe could be a blip
  expect(posted(team) == [])
  advance(10 min)  # ten more, one message
  expect(posted(team) == ["shop is down: it answered 503"])
  with fakes(http, from: "recordings/up/")
  advance(1 min)
  expect(posted(team) == ["shop is down: it answered 503", "shop is back after 11 min"])
}
# endregion test-outage

test "a single failed probe is not an outage" {
  let team = fakes(chat, from: "recordings/")
  with team
  with fakes(http, from: "recordings/down/")
  Site("shop").watch("https://shop.example.com")
  advance(1 min)
  with fakes(http, from: "recordings/up/")
  advance(1 h)
  expect(posted(team) == [])
}

test "a probe's answer is read as up or down" {
  expect(read(Ok({ status: 200, headers: {}, body: "" })) == Up)
  expect(read(Ok({ status: 404, headers: {}, body: "" })) == Up)
  expect(read(Err(TimedOut)) == Down(why: "TimedOut"))
}
`,Dy=`{
  "contract": "Chat",
  "exchanges": [
    { "op": "post", "key": "down-shop-1", "result": { "kind": "Posted", "id": "msg-1" } },
    { "op": "post", "key": "up-shop-1", "result": { "kind": "Posted", "id": "msg-2" } },
    { "op": "post", "key": "down-shop-2", "result": { "kind": "Posted", "id": "msg-3" } },
    { "op": "post", "key": "up-shop-2", "result": { "kind": "Posted", "id": "msg-4" } }
  ]
}
`,Ry=`{
  "contract": "http",
  "exchanges": [
    {
      "op": "get",
      "result": { "kind": "Ok", "value": { "status": 503, "headers": {}, "body": "Service Unavailable" } }
    }
  ]
}
`,Ey=`{
  "contract": "http",
  "exchanges": [
    { "op": "get", "result": { "kind": "Ok", "value": { "status": 200, "headers": {}, "body": "ok" } } }
  ]
}
`,Ly=`# The partner's webhook endpoint, as this program's contract describes it.
# The partner keeps the key of each event it has handled, and answers what
# it did with one when asked.

import http

## What the partner answers when it has an event.
pub type Ack = {
  ## The partner's id for what it received.
  id: String,
}

# region contract
@http(names: snake)
contract Endpoint {
  ## Delivers one event. The key goes in the Idempotency-Key header, and the
  ## partner handles each key once. A failure comes back as an Err at once,
  ## so the caller decides when to try again.
  @http("POST /webhooks")
  @key(required)
  @reconcile(receipt)
  command deliver(kind: String, body: String) -> Result(Ack, http.HttpError)

  ## What the partner did with the event sent with this key, or None if it
  ## never arrived.
  @http("GET /webhooks/:key")
  query receipt(key: Key) -> Option(Result(Ack, http.HttpError))
}
# endregion contract
`,By=`# Marks the root of an Eyelang program: imports name directories below this one.
`,_y=`# Webhook delivery. The shop tells a partner about each order event by
# calling the partner's webhook endpoint. Each event goes with a key, so the
# partner handles it once however often it arrives. A lost reply is settled
# by asking the partner. An endpoint that is down is tried again on a
# schedule this program writes for itself.

import endpoint
import http

## Something that happened that the partner wants to hear about.
pub type Event = { id: String, kind: String, body: String }

pub type Status =
  | Pending
  | Delivered(receipt: String)
  | GaveUp(why: String)

# region outgoing
## One event on its way to the partner.
entity Outgoing(id: String) {
  state status: Status = Pending

  ## Starts delivering an event, in one transaction. The run's key is the
  ## event's id, so queuing the same event again starts nothing.
  pub fn queue(e: Event) uses {start deliver} {
    deliver.start(e, key: "deliver-{id}")
  }

  pub fn finish(s: Status) { status = s }
}
# endregion outgoing

# region retry
## How long to wait after each failed try. The policy is this list: change
## it to change how often, and for how long, the program tries.
let backoff = [1 min, 5 min, 30 min, 2 h, 12 h]

## Whether another try could help: not after a success, and not after an
## answer that says no, such as 410 Gone.
fn worthRetrying(r: Result(a, http.HttpError)) -> Bool = match r {
  Ok(_) => false
  Err(BadStatus(code, _)) => code >= 500
  Err(_) => true
}

## Calls \`send\`, and calls it again after each wait in \`waits\` while the
## failure is worth retrying. Each sleep is a durable step, so a restart in
## the middle of a two-hour wait carries on where it was.
fn retry(waits: List(Duration), send: fn() -> Result(a, http.HttpError)) -> Result(a, http.HttpError) uses {wait} {
  var result = send()
  for d in waits {
    if !worthRetrying(result) { return result }
    sleep!(d)
    result = send()
  }
  result
}
# endregion retry

# region deliver
## Delivers one event. Every try sends the same key, so the partner acts on
## it once, even when an earlier try got through and only its reply was lost.
job deliver(e: Event) {
  let result = retry!(backoff, fn: endpoint.deliver!(e.kind, e.body, key: "{e.id}").reconcile!())
  let status = match result {
    Ok(ack) => Delivered(receipt: ack.id)
    Err(error) => GaveUp(why: "{error}")
  }
  Outgoing(e.id).finish!(status)
}
# endregion deliver

# ---------------------------------------------------------------- tests

let paid: Event = { id: "evt-1", kind: "order.paid", body: "\\{\\"order\\": 1182\\}" }

# region test-once
test "an event queued twice is delivered once" {
  let partner = fakes(endpoint, from: "recordings/")
  with partner
  Outgoing("evt-1").queue(paid)
  Outgoing("evt-1").queue(paid)
  expect(Outgoing("evt-1").status == Delivered(receipt: "wh-1"))
  expect(partner.calls(endpoint.deliver) |> size == 1)
}

test "a lost reply is settled by asking the partner, not by sending again" {
  let partner = fakes(endpoint, from: "recordings/")
  with partner
  Outgoing("evt-2").queue({ paid with id: "evt-2" })
  expect(Outgoing("evt-2").status == Delivered(receipt: "wh-2"))
  expect(partner.calls(endpoint.deliver) |> size == 1)
}

test "a lost reply the partner never got is sent again, with the same key" {
  let partner = fakes(endpoint, from: "recordings/")
  with partner
  Outgoing("evt-5").queue({ paid with id: "evt-5" })
  expect(Outgoing("evt-5").status == Delivered(receipt: "wh-5"))
  expect(partner.calls(endpoint.deliver) |> map(fn c: c.key) == ["evt-5", "evt-5"])
}
# endregion test-once

# region test-down
test "a partner that is down gets the event once it is back" {
  with clock(2026-10-12 09:00 in "UTC")
  with fakes(endpoint, from: "recordings/")  # three tries fail, the fourth gets through
  Outgoing("evt-4").queue({ paid with id: "evt-4" })
  advance(35 min)  # tries at 09:00, 09:01 and 09:06 failed
  expect(Outgoing("evt-4").status == Pending)
  advance(1 min)  # the try at 09:36 gets through
  expect(Outgoing("evt-4").status == Delivered(receipt: "wh-4"))
}

test "after the last wait, the delivery gives up and says why" {
  with fakes(endpoint, from: "recordings/down/")
  Outgoing("evt-1").queue(paid)
  advance(14 h + 35 min)
  expect(Outgoing("evt-1").status == Pending)
  advance(1 min)
  expect(Outgoing("evt-1").status == GaveUp(why: "BadStatus(code: 503, body: \\"down for maintenance\\")"))
}

test "an answer that says no is final" {
  let partner = fakes(endpoint, from: "recordings/")
  with partner
  Outgoing("evt-3").queue({ paid with id: "evt-3" })
  expect(Outgoing("evt-3").status == GaveUp(why: "BadStatus(code: 410, body: \\"unsubscribed\\")"))
  expect(partner.calls(endpoint.deliver) |> size == 1)
}
# endregion test-down
`,Py=`{
  "contract": "Endpoint",
  "exchanges": [
    {
      "op": "deliver",
      "key": "evt-1",
      "result": { "kind": "Err", "value": { "kind": "BadStatus", "code": 503, "body": "down for maintenance" } }
    }
  ]
}
`,Ny=`{
  "contract": "Endpoint",
  "exchanges": [
    { "op": "deliver", "key": "evt-1", "result": { "kind": "Ok", "value": { "id": "wh-1" } } },
    { "op": "deliver", "key": "evt-2", "result": { "kind": "Uncertain", "key": "evt-2" } },
    { "op": "receipt", "key": "evt-2", "result": { "kind": "Ok", "value": { "id": "wh-2" } } },
    {
      "op": "deliver",
      "key": "evt-3",
      "result": { "kind": "Err", "value": { "kind": "BadStatus", "code": 410, "body": "unsubscribed" } }
    },
    {
      "op": "deliver",
      "key": "evt-4",
      "result": { "kind": "Err", "value": { "kind": "BadStatus", "code": 503, "body": "down for maintenance" } }
    },
    {
      "op": "deliver",
      "key": "evt-4",
      "result": { "kind": "Err", "value": { "kind": "BadStatus", "code": 503, "body": "down for maintenance" } }
    },
    {
      "op": "deliver",
      "key": "evt-4",
      "result": { "kind": "Err", "value": { "kind": "BadStatus", "code": 503, "body": "down for maintenance" } }
    },
    { "op": "deliver", "key": "evt-4", "result": { "kind": "Ok", "value": { "id": "wh-4" } } },
    { "op": "deliver", "key": "evt-5", "result": { "kind": "Uncertain", "key": "evt-5" } },
    { "op": "receipt", "key": "evt-5", "result": null },
    { "op": "deliver", "key": "evt-5", "result": { "kind": "Ok", "value": { "id": "wh-5" } } }
  ]
}
`,Wy=`# Marks the root of an Eyelang program: imports name directories below this one.
`,qy=`# Support triage. Each message that arrives is sorted by a model into a
# category and an urgency, with a draft reply when the answer is simple.

import ai

pub type TicketId = | TicketId(String)

# region types
## How a support message is sorted.
pub type Triage = {
  category: Category,
  urgency: Urgency,
  ## One sentence, at most 20 words, saying what the customer wants.
  summary: String,
  ## A reply to send as it is, only when the answer is simple and certain.
  draft: Option(String),
}

pub type Category =
  | Bug
  | Question
  | Billing

pub type Urgency =
  | Normal
  ## Someone can't work, or is losing money, right now.
  | Urgent(reason: String)
# endregion types

pub type Sorting =
  | Unsorted
  | Sorted(Triage)
  ## The model failed; a person sorts this one.
  | ByHand(why: String)

entity Ticket(id: TicketId) {
  state text: String = ""
  state sorting: Sorting = Unsorted

  ## Takes a message in and starts its triage, in one transaction.
  pub fn receive(message: String) uses {start triage} {
    if text != "" { return }
    text = message
    triage.start(id, message, key: "triage-{id}")
  }

  pub fn sort(s: Sorting) { sorting = s }
}

# region ask
let instructions = """
  You sort messages sent to the support desk of a small web shop.
  Use only what the message says. Never promise refunds or dates.
  """

## Asks a model to sort a message. The type the result is used as, Triage,
## is the type the model is asked for.
fn sortMessage(text: String) -> Result(Triage, ai.AiError) {
  ai.generate!(model: "claude-sonnet-5-5", system: instructions, prompt: text)
}
# endregion ask

# region job
## Sorts one ticket. The model call is a recorded step: if the program
## restarts after it, the run goes on with the same answer and the model
## isn't asked, or paid, again.
job triage(id: TicketId, text: String) {
  match sortMessage!(text) {
    Ok(t) => Ticket(id).sort!(Sorted(t))
    Err(Malformed(_, problems)) => Ticket(id).sort!(ByHand(why: problems |> join("; ")))
    Err(e) => Ticket(id).sort!(ByHand(why: "{e}"))
  }
}
# endregion job

# region main
## Takes one message from the command line, for example
## \`eye run . -- t-1 "I was charged twice for order 1182"\`.
pub fn main(args: List(String)) -> Result((), String) uses {entity Ticket, start triage} {
  let (Some(id), Some(text)) = (args[1], args[2]) else {
    return Err("usage: eye run . -- TICKET MESSAGE")
  }
  Ticket(TicketId(id)).receive(text)
  Ok(())
}
# endregion main

# ---------------------------------------------------------------- tests

# region test-sorted
test "a recorded reply sorts the ticket" {
  with fakes(ai, from: "recordings/")
  let t = Ticket(TicketId("t-1"))
  t.receive("I was charged twice for order 1182.")
  let Sorted(triage) = t.sorting else { unreachable("the ticket should be sorted") }
  expect(triage.category == Billing)
  expect(triage.urgency == Urgent(reason: "the customer is losing money"))
  expect(triage.draft == None)
}
# endregion test-sorted

# region test-garbled
test "a reply that doesn't fit, twice, goes to a person" {
  with fakes(ai, from: "recordings/garbled/")
  let t = Ticket(TicketId("t-2"))
  t.receive("Where is my parcel?")
  expect(t.sorting == ByHand(why: "category: expected one of \\"Bug\\", \\"Question\\", \\"Billing\\", found \\"Shipping\\""))
}
# endregion test-garbled
`,Hy=`{
  "contract": "ai",
  "exchanges": [
    {
      "op": "generate",
      "result": {
        "category": "Billing",
        "urgency": { "kind": "Urgent", "reason": "the customer is losing money" },
        "summary": "The customer was charged twice for order 1182 and wants one charge back.",
        "draft": null
      }
    }
  ]
}
`,Fy=`{
  "contract": "ai",
  "exchanges": [
    {
      "op": "generate",
      "result": {
        "category": "Shipping",
        "urgency": { "kind": "Normal" },
        "summary": "The customer asks where their parcel is.",
        "draft": null
      }
    },
    {
      "op": "generate",
      "result": {
        "category": "Shipping",
        "urgency": { "kind": "Normal" },
        "summary": "The customer asks where their parcel is.",
        "draft": null
      }
    }
  ]
}
`,Vy=`# Marks the root of an Eyelang program: imports name directories below this one.
`,zy=`# The invoices program's own interface: the shop calls it to bill a customer.
# The program in the directory above serves it.

import payments.{ChargeId, CustomerId}

pub type InvoiceId = | InvoiceId(String)

## Where an invoice stands.
pub type Status =
  | Open
  | Paid(charge: ChargeId)
  | Unpaid(reason: String)
  | Refunded(amountCents: Int)

## What issuing an invoice did.
pub type Issue =
  | Issued(invoice: InvoiceId)
  | Refused(reason: String)

# region contract
@port(8080)
@http(names: snake)
contract Invoices {
  ## Bills a customer. The key names the invoice, so issuing it again with the
  ## same key bills the customer once and gives the same answer.
  @http("POST /v1/invoices")
  @key(required)
  @reconcile(issue)
  command issue(customer: CustomerId, amountCents: Int, phone: String) -> Issue

  ## Where an invoice stands, or None if there is no invoice with this id.
  @http("GET /v1/invoices/:invoice")
  query invoice(invoice: InvoiceId) -> Option(Status)
}
# endregion contract
`,jy=`# Invoices. Issuing an invoice charges the customer's card through the payment
# service, and texts them a receipt. Refunds arrive as events.

import http
import invoices.{InvoiceId, Status}
import payments.{ChargeId, CustomerId}
import sms

entity Invoice(id: InvoiceId) {
  state customer: Option(CustomerId) = None
  state amountCents: Int = 0
  state status: Status = Open
  ## Set when a text's reply was lost: a person checks whether it arrived.
  state receiptUnsure: Bool = false

  ## Records the invoice and starts collecting it, in one transaction.
  pub fn issue(c: CustomerId, cents: Int, phone: String) uses {start collect} {
    if customer != None { return }
    customer = Some(c)
    amountCents = cents
    collect.start(id, c, cents, phone, key: "collect-{id}")
  }

  pub fn settle(s: Status) { status = s }

  pub fn refund(cents: Int) { status = Refunded(amountCents: cents) }

  pub fn unsure() { receiptUnsure = true }
}

# region serve
## The program serves its own contract: one function per operation, each run
## as one transaction. The key names the invoice, so a repeated call finds
## the invoice it already made.
impl invoices.Invoices {
  fn issue(customer, amountCents, phone, key) {
    if amountCents <= 0 { return Refused(reason: "an invoice must be for more than 0") }
    let id = InvoiceId("{key}")
    Invoice(id).issue(customer, amountCents, phone)
    Issued(invoice: id)
  }

  fn invoice(invoice) = if Invoice(invoice).customer == None then None else Some(Invoice(invoice).status)
}
# endregion serve

# region collect
## Charges the customer once. The key is the invoice's id, so however often
## this step runs, the payment service takes the money once.
job collect(id: InvoiceId, customer: CustomerId, cents: Int, phone: String) {
  let charge = payments.charge!(customer, cents, "Invoice {id}", key: "invoice-{id}")
    .reconcile!()
  match charge {
    Charged(chargeId, _) => {
      Invoice(id).settle!(Paid(charge: chargeId))
      receipt!(id, phone, "Paid: invoice {id}, {cents / 100}.{cents % 100 |> pad2} EUR.")
    }
    Declined(reason) => Invoice(id).settle!(Unpaid(reason:))
  }
}
# endregion collect

fn pad2(n: Int) -> String = if n < 10 then "0{n}" else "{n}"

# region receipt
## Texts a receipt. The gateway takes no key, so a lost reply can't be
## settled by asking: a person looks instead, and nothing is sent twice.
fn receipt(id: InvoiceId, phone: String, text: String) {
  match sms.text!(phone, text) {
    Queued(_) | BadNumber => ()
    Uncertain(_) => Invoice(id).unsure!()
  }
}
# endregion receipt

# region refunded
## A refund made in the payment service's dashboard arrives as an event. Its
## @key is the charge, so a refund delivered twice runs this once.
@on(event(payments.refunded))
job refunded(r: payments.Refunded) {
  let paid = Invoice.all.read!() |> find(fn i: i.status == Paid(charge: r.charge))
  let Some(invoice) = paid else { return }
  Invoice(invoice.id).refund!(r.amountCents)
}
# endregion refunded

# region balance
## A customer's balance, or a message to show when the service can't say.
pub fn balanceText(c: CustomerId) -> String uses {net Payments.balance} {
  match payments.balance(c) {
    Ok(cents) => "{cents} cents"
    Err(BadStatus(code, _)) => "the payment service answered {code}"
    Err(NoAnswer(_)) | Err(TimedOut) => "the payment service didn't answer"
  }
}
# endregion balance

# region status-page
## Whether the payment service's public status page says all is well. The
## page has no contract, so it is read with http.get.
pub fn serviceUp() -> Bool uses {wait, net, env} {
  let url = env.get!("PAYMENTS_STATUS_URL") ?? "https://status.example.com"
  match http.get!(url, headers: { "Accept": "text/html" }) {
    Ok(page) => looksUp(page)
    Err(_) => false
  }
}

## Reads a status page's answer.
pub fn looksUp(page: http.Response) -> Bool =
  page.status == 200 && page.body |> contains("All systems operational")
# endregion status-page

# region main
## Issues one invoice from the command line, for example
## \`eye run . -- i-1 ann 4250 "+353 85 000 0001"\`.
pub fn main(args: List(String)) -> Result((), String) uses {entity Invoice, start collect} {
  let usage = "usage: eye run . -- INVOICE CUSTOMER CENTS PHONE"
  let (Some(id), Some(customer), Some(cents), Some(phone)) = (args[1], args[2], args[3], args[4]) else {
    return Err(usage)
  }
  let n = parseInt(cents).mapErr(fn _: usage)?
  Invoice(InvoiceId(id)).issue(CustomerId(customer), n, phone)
  Ok(())
}
# endregion main

# ---------------------------------------------------------------- tests

let ann = CustomerId("ann")

# region test-charged
test "a charge that goes through marks the invoice paid" {
  with fakes(payments, sms, from: "recordings/")
  let invoice = Invoice(InvoiceId("i-1"))
  invoice.issue(ann, 4_250, "+353 85 000 0001")
  expect(invoice.status == Paid(charge: ChargeId("ch-1")))
  expect(!invoice.receiptUnsure)
}
# endregion test-charged

# region test-declined
test "a declined card is an outcome, not an error" {
  with fakes(payments, sms, from: "recordings/")
  let invoice = Invoice(InvoiceId("i-2"))
  invoice.issue(CustomerId("bob"), 990, "+353 85 000 0002")
  expect(invoice.status == Unpaid(reason: "insufficient funds"))
}
# endregion test-declined

# region test-lost
test "a lost reply is settled by asking chargeFor, not by charging again" {
  with fakes(payments, sms, from: "recordings/")
  let invoice = Invoice(InvoiceId("i-3"))
  invoice.issue(CustomerId("cara"), 1_500, "+353 85 000 0003")
  expect(invoice.status == Paid(charge: ChargeId("ch-3")))
}
# endregion test-lost

# region test-unsure
test "a text whose reply was lost goes to a person, not out again" {
  with fakes(payments, from: "recordings/")
  with fakes(sms, from: "recordings/lost-text/")
  let invoice = Invoice(InvoiceId("i-1"))
  invoice.issue(ann, 4_250, "+353 85 000 0001")
  expect(invoice.status == Paid(charge: ChargeId("ch-1")))
  expect(invoice.receiptUnsure)
}
# endregion test-unsure

# region test-refund
test "a refund event marks the invoice refunded" {
  with fakes(payments, sms, from: "recordings/")
  Invoice(InvoiceId("i-1")).issue(ann, 4_250, "+353 85 000 0001")
  payments.refunded.publish({ charge: ChargeId("ch-1"), amountCents: 4_250 })
  expect(Invoice(InvoiceId("i-1")).status == Refunded(amountCents: 4_250))
}
# endregion test-refund

# region test-balance
test "a failure the query declares comes back as a value" {
  with fakes(payments, from: "recordings/")
  expect(balanceText(ann) == "the payment service answered 503")
}
# endregion test-balance

# region test-serve
test "the served contract bills once per key" {
  with fakes(payments, sms, from: "recordings/")
  let first = invoices.issue(ann, 4_250, "+353 85 000 0001", key: "i-1")
  let again = invoices.issue(ann, 4_250, "+353 85 000 0001", key: "i-1")
  expect(first == Issued(invoice: InvoiceId("i-1")))
  expect(again == first)
  expect(invoices.invoice(InvoiceId("i-1")) == Some(Paid(charge: ChargeId("ch-1"))))
  expect(invoices.invoice(InvoiceId("nope")) == None)
  expect(invoices.issue(ann, 0, "+353 85 000 0001", key: "i-9") == Refused(reason: "an invoice must be for more than 0"))
}
# endregion test-serve

# region test-json
type Reading = { at: Instant, every: Duration, note: Option(String), level: Level }

type Level =
  | Calm
  | Busy

test "values have one JSON shape" {
  let paid: payments.Charge = Charged(id: ChargeId("ch-1"), at: 2026-10-12 09:30 in "UTC")
  expect(paid |> toJson == "\\{\\"at\\":\\"2026-10-12T09:30:00Z\\",\\"id\\":\\"ch-1\\",\\"kind\\":\\"Charged\\"\\}")
  let reading: Reading = { at: 2026-10-12 09:30 in "UTC", every: 90 s, note: None, level: Busy }
  expect(reading |> toJson == "\\{\\"at\\":\\"2026-10-12T09:30:00Z\\",\\"every\\":90000,\\"level\\":\\"Busy\\",\\"note\\":null\\}")
  expect(Some(3) |> toJson == "3")
  expect([Calm, Busy] |> toJson == "[\\"Calm\\",\\"Busy\\"]")
}
# endregion test-json

# region test-page
test "a status page is read from its status and body" {
  let page: http.Response = { status: 200, headers: {}, body: "<p>All systems operational</p>" }
  expect(looksUp(page))
  expect(!looksUp({ page with status: 503 }))
}
# endregion test-page
`,Uy=`# The payment service takes money from customers' cards. This contract is its
# interface: the invoices program calls it.

import http

pub type CustomerId = | CustomerId(String)

pub type ChargeId = | ChargeId(String)

# region types
## What the payment service did with a charge.
pub type Charge =
  ## The money was taken.
  | Charged(id: ChargeId, at: Instant)
  ## The card was declined, and nothing was taken.
  | Declined(reason: String)

## Money given back to a customer, in part or in full.
pub type Refunded = { charge: ChargeId, amountCents: Int }
# endregion types

# region contract
@http(names: snake)
@auth(bearer: "payments-token")
contract Payments {
  ## Takes money from a customer's card. The service keeps what each key did.
  @http("POST /v1/charges", status: { 201: Charged, 402: Declined })
  @key(required)
  @reconcile(chargeFor)
  command charge(customer: CustomerId, amountCents: Int, description: String) -> Charge

  ## What the charge with this key did, or None if it never arrived.
  @http("GET /v1/charges/by-key/:key")
  query chargeFor(key: Key) -> Option(Charge)

  ## A customer's balance in cents. A failure comes back as an Err instead
  ## of being retried.
  @http("GET /v1/customers/:customer/balance")
  query balance(customer: CustomerId) -> Result(Int, http.HttpError)

  @key(charge)
  event refunded: Refunded on "payments.refunds"
}
# endregion contract
`,$y=`{
  "contract": "Sms",
  "exchanges": [{ "op": "text", "result": { "kind": "Uncertain", "key": "text-1" } }]
}
`,Ky=`{
  "contract": "Payments",
  "exchanges": [
    {
      "op": "charge",
      "key": "invoice-i-1",
      "result": { "kind": "Charged", "id": "ch-1", "at": "2026-10-12T09:30:00Z" }
    },
    {
      "op": "charge",
      "key": "invoice-i-2",
      "result": { "kind": "Declined", "reason": "insufficient funds" }
    },
    { "op": "charge", "key": "invoice-i-3", "result": { "kind": "Uncertain", "key": "invoice-i-3" } },
    {
      "op": "chargeFor",
      "key": "invoice-i-3",
      "result": { "kind": "Charged", "id": "ch-3", "at": "2026-10-12T09:31:00Z" }
    },
    {
      "op": "balance",
      "result": { "kind": "Err", "value": { "kind": "BadStatus", "code": 503, "body": "down for maintenance" } }
    }
  ]
}
`,Gy=`{
  "contract": "Sms",
  "exchanges": [{ "op": "text", "result": { "kind": "Queued", "id": "sm-1" } }]
}
`,Yy=`# A text message gateway. It takes no idempotency key and can't say what it
# did with a request, so a lost reply can't be reconciled.

pub type Sent =
  | Queued(id: String)
  | BadNumber

# region contract
contract Sms {
  ## Sends a text. Sending it again sends a second text.
  @http("POST /messages")
  command text(to: String, body: String) -> Sent
}
# endregion contract
`,Jy=`# Marks the root of an Eyelang program: imports name directories below this one.
`,Xy=`## Courier pickups: which pickup a parcel packed now can still make.

# region types
## A courier's pickup at the warehouse.
pub type Pickup = {
  ## When the courier arrives, as booked.
  arrives: Instant,
  ## How long before that the parcels must be packed.
  cutoff: Duration,
}

## How a delivery went.
pub type Delivery =
  ## Delivered on the promised day.
  | OnTime
  ## Delivered late, by this many days.
  | Late(Int)
# endregion types

# region next
## The first pickup in \`pickups\` that a parcel packed at \`now\` can still
## make, if there is one.
##
## A parcel must be packed before the pickup's cutoff. See [Pickup.cutoff].
pub fn next(pickups: List(Pickup), now: Instant) -> Option(Pickup) =
  pickups |> find(fn p: p.arrives - p.cutoff > now)
# endregion next

test "a pickup past its cutoff is missed" {
  with clock(2026-10-12 16:00 in "UTC")
  let missed = { arrives: now() + 20 min, cutoff: 30 min }
  let later = { arrives: now() + 2 h, cutoff: 30 min }
  expect(next([missed, later], now()) == Some(later))
  expect(next([missed], now()) == None)
}
`,Qy=`# The team's chat. Posting keeps what each key did, so posting again with the
# same key is how a lost reply is settled.

pub type Post =
  | Posted(id: String)
  | Refused(error: String)

pub type Message = { by: String, text: String }

contract Chat {
  ## Posts a message to the team's channel.
  @key(required)
  @reconcile(post)
  command post(text: String) -> Post

  ## The channel's latest messages.
  query history(limit: Int) -> List(Message)
}
`,Zy=`# Marks the root of an Eyelang program: imports name directories below this one.
`,eb=`# Late orders. A shop's orders, a rule for when one is late, and a job that
# tells the team about late orders every evening.

import chat

let dublin: Zone = "Europe/Dublin"

pub type OrderId = | OrderId(String)

entity Order(id: OrderId) {
  state placed: Option(Instant) = None
  state shipped: Bool = false
  state flagged: Bool = false

  pub fn place(at: Instant) {
    if placed == None { placed = Some(at) }
  }

  pub fn ship() { shipped = true }

  pub fn flag() { flagged = true }
}

# region pure
## Whether an order is late: not shipped two days after it was placed. It
## takes the time as an argument, so it is pure, and a test can pass any time.
pub fn isLate(placed: Instant, shipped: Bool, at: Instant) -> Bool =
  !shipped && at - placed > 2 days
# endregion pure

# region uses
## The orders that are late now.
pub fn lateOrders() -> List(OrderId) uses {clock, entity Order} {
  let at = now()
  Order.all
    |> filter(fn o: match o.placed {
      Some(placed) => isLate(placed, o.shipped, at)
      None => false
    })
    |> map(fn o: o.id)
}
# endregion uses

# region inferred
## The line the team reads. Private and pure.
fn summary(late: List(OrderId)) -> String =
  "{late |> size} orders are late: {late |> map(fn id: "{id}") |> join(", ")}"

## Posts to the team's channel. Private, so its effects are inferred:
## wait and net Chat.post.
@unwitnessed("Chat refuses only when the channel is archived, which no recording has")
fn post(text: String, key: Key) {
  match chat.post!(text, key:).reconcile!() {
    Posted(_) => ()
    Refused(error) => log.warn("the chat refused {key}: {error}")
  }
}

## Tells the team which orders are late. Public, so it states what post does.
pub fn tellLate(late: List(OrderId), key: Key) uses {wait, net Chat.post} {
  if late == [] { return }
  post!(summary(late), key:)
}
# endregion inferred

# region callback
## Calls \`f\` with each late order. It states only its own effects; a caller
## adds the effects of the function it passes.
pub fn eachLate(f: fn(OrderId)) uses {clock, entity Order} {
  for id in lateOrders() { f(id) }
}

## Prints the late orders. The print is in the lambda, so stdio is this
## function's effect, not eachLate's.
pub fn printLate() uses {clock, entity Order, stdio} {
  eachLate(fn id: print("late: {id}"))
}
# endregion callback

# region job
## Every evening, flags the late orders and tells the team. Each \`!\` is a
## step whose result is saved, so a restart repeats none of them.
@on(daily(18:00, in: dublin))
job evening(day: Date) {
  let late = lateOrders!()
  for id in late { Order(id).flag!() }
  tellLate!(late, key: "late-{day}")
}
# endregion job

# ---------------------------------------------------------------- tests

# region tests
test "an order is late two days after it was placed, until it ships" {
  let placed = 2026-10-12 10:00 in dublin
  expect(!isLate(placed, false, placed + 47 h))
  expect(isLate(placed, false, placed + 49 h))
  expect(!isLate(placed, true, placed + 49 h))
}

test "every evening the team hears about late orders" {
  with fakes(chat, from: "recordings/")
  with clock(2026-10-12 09:00 in dublin)
  Order(OrderId("o-1")).place(now())
  Order(OrderId("o-2")).place(now())
  Order(OrderId("o-2")).ship()
  advance(2 days)  # the evening of the 12th and the 13th find nothing late
  expect(!Order(OrderId("o-1")).flagged)
  advance(9 h)  # 18:00 on the 14th
  expect(Order(OrderId("o-1")).flagged)
  expect(!Order(OrderId("o-2")).flagged)
  expect(lateOrders() == [OrderId("o-1")])
  printLate()  # prints "late: o-1"
}
# endregion tests
`,tb=`{
  "contract": "Chat",
  "exchanges": [{ "op": "post", "key": "late-2026-10-14", "result": { "kind": "Posted", "id": "msg-1" } }]
}
`,nb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,ib=`# Guide: entities. Prepaid credit for model calls, one wallet per team.

# region wallet
type SpendError = | Short(balance: Int)

## A team's prepaid credit, in cents.
entity Wallet(team: String) {
  state cents: Int = 0
  state spent: Int = 0
  state warned: Bool = false

  pub fn topUp(n: Int) {
    cents += n
    warned = false
  }

  pub fn spend(n: Int) -> Result((), SpendError) {
    if n > cents { return Err(Short(balance: cents)) }
    cents -= n
    spent += n
    Ok(())
  }
}
# endregion wallet

# region test-wallet
test "a wallet keeps its state between calls" {
  let ops = Wallet("ops")
  ops.topUp(1000)
  expect(ops.spend(300) == Ok(()))
  expect(ops.spend(800) == Err(Short(balance: 700)))
  expect(ops.cents == 700 && ops.spent == 300)
}
# endregion test-wallet

# region identity
test "the identity is the entity" {
  Wallet("ops").topUp(200)
  expect(Wallet("ops").cents == 200)
  expect(Wallet("research").cents == 0)  # exists from its first use
}
# endregion identity

# region ledger
## Moves credit from one team to another. Both wallets change in one
## transaction, or neither does.
entity Ledger(name: String) {
  state moves: Int = 0

  pub fn move(from: Ref(Wallet), to: Ref(Wallet), n: Int) -> Result((), SpendError) uses {entity Wallet} {
    from.spend(n)?
    to.topUp(n)
    moves += 1
    Ok(())
  }
}
# endregion ledger

# region test-ledger
test "nested calls commit together" {
  let ops = Wallet("ops")
  let research = Wallet("research")
  ops.topUp(1000)
  expect(Ledger("main").move(ops, research, 400) == Ok(()))
  expect(ops.cents == 600 && research.cents == 400)
  expect(Ledger("main").move(research, ops, 900) == Err(Short(balance: 400)))
  expect(Ledger("main").moves == 1)
}
# endregion test-ledger

# region snapshot
## What a team sees on its dashboard. It takes a snapshot, a plain value, so
## it can't change the wallet.
fn describe(w: Wallet) -> String {
  let left = "{w.team}: {w.cents} cents left"
  if w.spent == 0 { left } else { "{left}, {w.spent} spent" }
}
# endregion snapshot

# region test-snapshot
test "a snapshot doesn't change when the entity does" {
  let ops = Wallet("ops")
  ops.topUp(500)
  let before = ops.read()
  let _ = ops.spend(200)
  expect(before.cents == 500 && before.version == 1)
  expect(ops.read().version == 2)
  expect(describe(before) == "ops: 500 cents left")
  expect(describe(ops.read()) == "ops: 300 cents left, 200 spent")
}
# endregion test-snapshot

# region quota
## Like Wallet, but it warns the team once when it runs low.
entity Quota(team: String) {
  state cents: Int = 0
  state warned: Bool = false

  pub fn topUp(n: Int) {
    cents += n
    warned = false
  }

  pub fn spend(n: Int) -> Result((), SpendError) uses {start warnLow} {
    if n > cents { return Err(Short(balance: cents)) }
    cents -= n
    if cents < 100 && !warned {
      warned = true
      warnLow.start(team)
    }
    Ok(())
  }
}

job warnLow(team: String) {
  let q = Quota(team).read!()
  Outbox(team).add!("only {q.cents} cents left")
}

## The messages a team was sent. A real program would send them with a
## contract; this one keeps them so the tests can look.
entity Outbox(team: String) {
  state sent: List(String) = []

  pub fn add(text: String) { sent += [text] }
}
# endregion quota

# region test-quota
test "a method starts a job when it commits" {
  let q = Quota("ops")
  q.topUp(500)
  let _ = q.spend(450)
  let _ = q.spend(20)
  expect(Outbox("ops").sent == ["only 50 cents left"])
}
# endregion test-quota

# region close
## A team's access to the models. Closing it deletes its data, so it needs
## a person's approval.
entity Team(name: String) {
  state members: List(String) = []

  pub fn join(who: String) { members += [who] }

  pub fn close(ok: Approval) { end(ok) }
}

job closeTeam(name: String) {
  let decision = approval!("Close team {name}?", from: "finance", within: 2 days)
  let Approved(ok) = decision else { return }
  Team(name).close!(ok)
}
# endregion close

# region test-close
test "a team ends once finance approves" {
  Team("ops").join("ann")
  closeTeam.start("ops")
  approve("Close team ops?")
  expect(Team.all |> size == 0)
}
# endregion test-close

# region all
test "Wallet.all is every wallet" {
  Wallet("ops").topUp(50)
  Wallet("research").topUp(5000)
  let low = Wallet.all |> filter(fn w: w.cents < 100) |> map(fn w: w.team)
  expect(low == ["ops"])
}
# endregion all

# region versions
## Version 1 kept whole credits as \`credits\`; version 2 keeps cents.
@version(2)
entity Account(team: String) {
  state cents: Int = 0
  ## Added later with a default: needs no new version.
  state frozen: Bool = false

  pub fn add(n: Int) { cents += n }

  migrate from 1 (old) {
    { cents: old.credits * 100 }
  }
}
# endregion versions

test "a new account starts at the current version" {
  Account("ops").add(250)
  expect(Account("ops").cents == 250 && !Account("ops").frozen)
}
`,sb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,rb=`# Guide: jobs and waiting. Reminders that ring at their time, exports that
# must finish by a deadline, and a clean-up that asks a person first.

## What the runs wrote down, so the tests can look. A real program would send
## these lines through a contract instead.
entity Log(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

# region reminder
entity Reminder(id: String) {
  state text: String = ""
  state due: Instant = 2026-01-01 00:00 in "UTC"

  pub fn set(t: String, at: Instant) uses {start ring} {
    text = t
    due = at
    ring.start(id, key: "ring-{id}")
  }

  pub fn snooze(by: Duration) { due = due + by }
}
# endregion reminder

# region ring
## Rings a reminder when it is due. A snooze moves the time while the run
## sleeps, so it reads the reminder again after every wait.
job ring(id: String) {
  var me = Reminder(id).read!()
  while me.due > now!() {
    sleepUntil!(me.due)
    me = Reminder(id).read!()
  }
  Log("rang").add!("{me.text} at {now!().time(in: "Europe/Dublin")}")
}
# endregion ring

# region test-ring
test "a reminder rings at its time, and a snooze moves it" {
  with clock(2026-10-12 09:00 in "Europe/Dublin")
  Reminder("tea").set("put the kettle on", 2026-10-12 10:00 in "Europe/Dublin")
  Reminder("call").set("call the plumber", 2026-10-12 11:00 in "Europe/Dublin")
  advance(1 h)
  expect(Log("rang").lines == ["put the kettle on at 10:00"])
  Reminder("call").snooze(30 min)
  advance(2 h)
  expect(Log("rang").lines == ["put the kettle on at 10:00", "call the plumber at 11:30"])
}
# endregion test-ring

# region test-key
test "setting a reminder twice starts one run" {
  with clock(2026-10-12 09:00 in "Europe/Dublin")
  let r = Reminder("tea")
  r.set("put the kettle on", 2026-10-12 10:00 in "Europe/Dublin")
  r.set("put the kettle on, now", 2026-10-12 10:15 in "Europe/Dublin")
  advance(1 day)
  expect(Log("rang").lines == ["put the kettle on, now at 10:15"])
}
# endregion test-key

# region prune
## Deletes old backups, but only if a person says yes within a day.
job prune(old: Int) {
  match approval!("delete {old} old backups", from: "operator", within: 1 day) {
    Approved(ok) => Log("backups").add!("deleted {old}, approved by {ok.by}")
    Declined => Log("backups").add!("kept {old}: declined")
    Expired => Log("backups").add!("kept {old}: nobody answered")
  }
}
# endregion prune

# region test-prune
test "a person approves the clean-up" {
  prune.start(3)
  expect(Log("backups").lines == [])
  approve("delete 3 old backups", by: "someone else")
  expect(Log("backups").lines == [])  # only the person asked may answer
  approve("delete 3 old backups")
  expect(Log("backups").lines == ["deleted 3, approved by operator"])
}

test "nobody answers, so nothing is deleted" {
  prune.start(3)
  advance(1 day)
  expect(Log("backups").lines == ["kept 3: nobody answered"])
}
# endregion test-prune

# region export
type Table = { name: String, minutes: Int }

## Exports one table. The sleep stands in for slow work in another service.
@deadline(30 min)
job export(t: Table) -> Int {
  sleep!(1 min * t.minutes)
  t.minutes * 1000
}

## Starts one export per table, then waits for each. The exports run at
## once; one that passes its deadline is reported, not lost.
job nightly(tables: List(Table)) {
  var runs = []
  for t in tables { runs += [(t.name, export.start!(t))] }
  for (name, run) in runs {
    match run.join!() {
      Completed(rows) => Log("nightly").add!("{name}: {rows} rows")
      Failed(fault) => Log("nightly").add!("{name}: {fault}")
    }
  }
}
# endregion export

# region test-export
test "child runs go at once, and a fault comes back as an outcome" {
  let start = now()
  nightly.start([{ name: "users", minutes: 5 }, { name: "events", minutes: 45 }, { name: "logins", minutes: 20 }])
  advance(1 h)
  expect(Log("nightly").lines == ["users: 5000 rows", "events: DeadlinePassed", "logins: 20000 rows"])
  expect(now() - start == 1 h)
}
# endregion test-export

# region post
## Posts to a chat. The chat service allows one message a second in each
## chat, and the runtime keeps to that.
@limit(1, per: 1 s, by: chat)
job post(chat: String, text: String) {
  Log(chat).add!(text)
}

## Rebuilds a chat's search index, one rebuild at a time per chat.
@concurrency(1, by: chat)
job reindex(chat: String) {
  Log("reindex").add!("start {chat}")
  sleep!(10 min)
  Log("reindex").add!("end {chat}")
}
# endregion post

# region test-post
test "@limit spaces runs out, per chat" {
  post.start("team", "one")
  post.start("team", "two")
  post.start("family", "hello")
  expect(Log("team").lines == ["one"] && Log("family").lines == ["hello"])
  advance(1 s)
  expect(Log("team").lines == ["one", "two"])
}

test "@concurrency queues runs, per chat" {
  reindex.start("team")
  reindex.start("team")
  reindex.start("family")
  advance(10 min)
  expect(Log("reindex").lines == ["start team", "start family", "end team", "start team", "end family"])
}
# endregion test-post

# region first
job ask(what: String) -> Decision { approval!(what, from: "operator", within: 3 days) }

job timer(d: Duration) -> Decision {
  sleep!(d)
  Expired
}

## Restarts a server if someone approves within the hour.
job restart(server: String) {
  match firstDone!([ask.start!("Restart {server}?"), timer.start!(1 h)]) {
    Completed(Approved(_)) => Log("restarts").add!("{server} restarted")
    Completed(_) | Failed(_) => Log("restarts").add!("{server} left alone")
  }
}
# endregion first

# region test-first
test "firstDone! takes whichever run finishes first" {
  restart.start("db")
  restart.start("web")
  approve("Restart db?")
  advance(1 h)
  expect(Log("restarts").lines == ["db restarted", "web left alone"])
}
# endregion test-first

# region lock
## Migrates the database. Two migrations must never run at once, so each
## holds the lock "db" while it works.
job migrate(name: String) {
  lock!("db", for: 1 h, fn {
    Log("db").add!("{name} starts")
    sleep!(10 min)
    Log("db").add!("{name} ends")
  })
}
# endregion lock

# region test-lock
test "a lock lets one migration run at a time" {
  migrate.start("a")
  migrate.start("b")
  advance(30 min)
  expect(Log("db").lines == ["a starts", "a ends", "b starts", "b ends"])
}
# endregion test-lock

# region main
## Sets one reminder the first time the program starts.
pub fn main() uses {clock, entity Reminder, start ring} {
  let r = Reminder("stretch")
  if r.read().version == 0 {
    r.set("stand up and stretch", now() + 1 min)
  }
}
# endregion main
`,ob=`# Marks the root of an Eyelang program: imports name directories below this one.
`,ab=`# Writes the digest's text. Pure functions, tested where they are declared.

# region headline
## The first line of a digest: the day, and how many items it holds.
pub fn headline(day: Date, items: Int) -> String {
  let what = if items == 1 { "1 item" } else { "{items} items" }
  "{day.format("%A %-d %B")}: {what}"
}

test "a headline names the day and counts the items" {
  expect(headline(2026-10-12, 3) == "Monday 12 October: 3 items")
  expect(headline(2026-10-13, 1) == "Tuesday 13 October: 1 item")
}
# endregion headline
`,lb=`# Guide: testing. A morning digest: the items that came in overnight, sent to
# one person through Notify at 07:30, Irish time.

import notify
import format.{headline}

let dublin: Zone = "Europe/Dublin"

# region inbox
## What came in since the last digest, and what was sent.
entity Inbox(owner: String) {
  state items: List(String) = []
  state sent: List(String) = []

  pub fn add(item: String) { items += [item] }

  pub fn delivered(id: String) {
    sent += [id]
    items = []
  }
}
# endregion inbox

# region digest
## Sends the morning digest, if anything came in.
@on(daily(07:30, in: dublin))
job digest(day: Date) {
  let inbox = Inbox("ann").read!()
  if inbox.items == [] { return }
  let text = "{headline(day, inbox.items |> size)}\\n{inbox.items |> join("\\n")}"
  match notify.send!(to: "ann", text:, key: "digest-{day}").reconcile!() {
    Accepted(id) => Inbox("ann").delivered!(id)
    Refused(error) => log.warn("Notify refused the digest for {day}: {error}")
  }
}
# endregion digest

# region test-digest
test "the digest goes out at 07:30" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-12 06:00 in dublin)
  Inbox("ann").add("Rail strike called off")
  advance(89 min)
  expect(Inbox("ann").sent == [])
  advance(1 min)
  expect(Inbox("ann").sent == ["m-1"])
  expect(Inbox("ann").items == [])
}
# endregion test-digest

# region test-fresh
test "each test starts with an empty store" {
  expect(Inbox("ann").items == [] && Inbox("ann").sent == [])
}
# endregion test-fresh

# region test-lost
test "a lost reply is settled by asking Notify, not by sending again" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-13 07:00 in dublin)
  Inbox("ann").add("Storm warning for Tuesday")
  advance(30 min)
  expect(Inbox("ann").sent == ["m-2"])
}
# endregion test-lost

# region test-refused
test "a refused digest keeps the items for tomorrow" {
  with fakes(notify, from: "recordings/")
  with clock(2026-10-14 07:00 in dublin)
  Inbox("ann").add("Bin day moved")
  advance(1 h)
  expect(Inbox("ann").sent == [] && Inbox("ann").items == ["Bin day moved"])
}
# endregion test-refused

# region deadline
## Waits for the feed's morning fetch, which sometimes never ends.
@deadline(1 h)
job fetch(feed: String) -> Int {
  sleep!(2 h)
  0
}
# endregion deadline

# region test-deadline
test "a run that passes its deadline ends as an outcome" {
  let run = fetch.start("ireland")
  advance(59 min)
  expect(run.outcome() == None)
  advance(1 min)
  expect(run.outcome() == Some(Failed(DeadlinePassed)))
}
# endregion test-deadline
`,hb=`# Notify sends messages to people in their chat. This contract is its
# interface; the digest calls it.

## What Notify did with a message.
pub type Delivery =
  ## The outbox has it. A repeated key gives the first answer again.
  | Accepted(id: String)
  ## A bad request, or a person Notify doesn't know.
  | Refused(error: String)

# region contract
contract Notify {
  ## Sends a message to one person. Each key is sent at most once.
  @key(required)
  @reconcile(sent)
  command send(to: String, text: String) -> Delivery

  ## What happened to the message sent with this key, if it arrived.
  query sent(key: Key) -> Option(Delivery)
}
# endregion contract
`,cb=`{
  "contract": "Notify",
  "exchanges": [
    { "op": "send", "key": "digest-2026-10-12", "result": { "kind": "Accepted", "id": "m-1" } },
    { "op": "send", "key": "digest-2026-10-13", "result": { "kind": "Uncertain", "key": "digest-2026-10-13" } },
    { "op": "sent", "key": "digest-2026-10-13", "result": { "kind": "Accepted", "id": "m-2" } },
    { "op": "send", "key": "digest-2026-10-14", "result": { "kind": "Refused", "error": "no chat for ann" } }
  ]
}
`,db=`# Marks the root of an Eyelang program: imports name directories below this one.
`,ub=`# Guide: views, events and schedules. Disks report how full they are; a view
# finds the nearly full ones; jobs page someone and wait for an answer; a
# schedule sends a morning summary.

import monitor.{Reading}

let dublin: Zone = "Europe/Dublin"

## Who was told what, so the tests can look. A real program would send these
## lines through a contract.
entity Pager(name: String) {
  state lines: List(String) = []

  pub fn add(line: String) { lines += [line] }
}

# region disk
entity Disk(host: String) {
  state used: Int = 0

  pub fn report(percent: Int) { used = percent }

  ## Someone has seen the alert: tell every run that waits for it.
  pub fn acknowledge(by: String) uses {publish Monitor.acks} {
    monitor.acks.publish({ host, by })
  }
}
# endregion disk

# region record
## Each reading the monitor sends starts one run, and updates its disk.
@on(event(monitor.readings))
job record(r: Reading) {
  Disk(r.host).report!(r.used)
}
# endregion record

# region view
## Whether a disk is 90% full or more.
fn full(d: Disk) -> Bool = d.used >= 90

## Every disk that is nearly full.
view nearlyFull = Disk.all |> filter(full)
# endregion view

# region triggers
## Pages the on-call person when a disk becomes nearly full, and tells the
## lead if nobody acknowledges it within an hour.
@on(enters(nearlyFull))
job page(d: Ref(Disk)) {
  let host = d.host
  Pager("on-call").add!("{host} is nearly full")
  let ack = next!(monitor.acks, where: fn a: a.host == host, within: 1 h)
  match ack {
    Some(a) => Pager("on-call").add!("{a.by} is on {host}")
    None => Pager("lead").add!("nobody answered for {host}")
  }
}

## Says when a disk is back to normal.
@on(leaves(nearlyFull))
job settle(d: Ref(Disk)) {
  Pager("on-call").add!("{d.host} is fine again")
}
# endregion triggers

# region test-view
test "a disk that fills up enters the view, and leaves it when cleaned" {
  monitor.readings.publish({ id: 1, host: "nas", used: 80 })
  expect(Pager("on-call").lines == [])
  monitor.readings.publish({ id: 2, host: "nas", used: 93 })
  monitor.readings.publish({ id: 3, host: "nas", used: 95 })
  expect(Pager("on-call").lines == ["nas is nearly full"])
  Disk("nas").acknowledge("ann")
  monitor.readings.publish({ id: 4, host: "nas", used: 60 })
  expect(Pager("on-call").lines == ["nas is nearly full", "ann is on nas", "nas is fine again"])
}
# endregion test-view

# region test-events
test "a reading sent twice starts one run" {
  monitor.readings.publish({ id: 1, host: "nas", used: 95 })
  monitor.readings.publish({ id: 1, host: "nas", used: 95 })
  expect(Pager("on-call").lines == ["nas is nearly full"])
}

test "nobody acknowledges, so the lead hears about it" {
  monitor.readings.publish({ id: 1, host: "nas", used: 95 })
  advance(1 h)
  expect(Pager("lead").lines == ["nobody answered for nas"])
}
# endregion test-events

# region morning
## On weekday mornings, a summary of the disks that are nearly full.
@on(weekdays(08:30, in: dublin))
job morning(day: Date) {
  let hosts = nearlyFull.read!() |> map(fn d: d.host)
  if hosts == [] { return }
  Pager("team").add!("{day.format("%A")}: {hosts |> join(", ")} nearly full")
}
# endregion morning

# region compact
## Compacts the disks every night at 01:30, Irish time.
@on(daily(01:30, in: dublin))
job compact(day: Date) {
  Pager("night").add!("{day} at {now!()}")
}
# endregion compact

# region rotate
## Rotates the logs on the hour, every hour: on the night the clock goes back,
## the hour it repeats is rotated twice.
@on(cron("0 * * * *", in: dublin, onOverlap: Both))
job rotate() {
  Pager("logs").add!("{now!()}")
}
# endregion rotate

# region test-schedules
test "the summary comes on weekday mornings only" {
  with clock(2026-10-16 08:00 in dublin)  # a Friday
  Disk("nas").report(97)
  advance(3 days + 30 min)
  expect(Pager("team").lines == ["Friday: nas nearly full", "Monday: nas nearly full"])
}

test "a time the clock skips runs as far after the change" {
  with clock(2027-03-27 12:00 in dublin)  # at 01:00 on the 28th, the clock jumps to 02:00
  advance(2 days)
  expect(Pager("night").lines == [
    "2027-03-28 at 2027-03-28 02:30 in \\"Europe/Dublin\\"",
    "2027-03-29 at 2027-03-29 01:30 in \\"Europe/Dublin\\"",
  ])
}

test "a time the clock repeats runs twice with onOverlap: Both" {
  # 00:30 in Dublin; at 02:00 on summer time, 01:00 UTC, the clock goes back to 01:00
  with clock(2026-10-24 23:30 in "UTC")
  advance(3 h)
  expect(Pager("logs").lines == [
    "2026-10-25 00:00 in \\"UTC\\"",  # 01:00 on summer time
    "2026-10-25 01:00 in \\"UTC\\"",  # 01:00 again, on winter time
    "2026-10-25 02:00 in \\"UTC\\"",
  ])
}
# endregion test-schedules
`,fb=`# The monitoring service: it sends a reading of each disk every few minutes,
# and an acknowledgement when someone has seen an alert.

## How full one disk is, in percent.
pub type Reading = { id: Int, host: String, used: Int }

## Who has seen the alert for which host.
pub type Ack = { host: String, by: String }

# region contract
@auth(bearer: "monitor-token")
contract Monitor {
  ## One reading of one disk. A reading sent twice has the same id.
  @key(id)
  event readings: Reading on "monitor.readings"

  ## Someone has seen the alert for a host.
  event acks: Ack on "monitor.acks"
}
# endregion contract
`,pb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,mb=`# A mail service. This contract is its interface; the invoices program calls it.

## What the service did with a message.
pub type Delivery =
  ## The service has it. A repeated key gives the first answer again.
  | Sent(id: String)
  ## An address the service can't deliver to.
  | Bounced(reason: String)

# region contract
contract Mail {
  ## Sends one message. Each key is sent at most once.
  @key(required)
  @reconcile(sent)
  command send(to: String, text: String) -> Delivery

  ## What happened to the message sent with this key, if it arrived.
  query sent(key: Key) -> Option(Delivery)
}
# endregion contract
`,gb=`# Invoices. When an invoice goes out, the customer gets one reminder three
# days later, unless they have paid by then.

import mail

# region invoice
entity Invoice(id: String) {
  state email: String = ""
  state paid: Bool = false

  ## Sends the invoice and starts its reminder, in one transaction.
  pub fn issue(to: String) uses {start remind} {
    email = to
    remind.start(id, key: "remind-{id}")
  }

  pub fn pay() { paid = true }
}
# endregion invoice

# region job
## Waits three days, then reminds the customer unless they have paid.
job remind(id: String) {
  sleep!(3 days)
  let invoice = Invoice(id).read!()
  if invoice.paid { return }
  let text = "Invoice {id} is still open."
  match mail.send!(to: invoice.email, text:, key: "remind-{id}").reconcile!() {
    Sent(_) => ()
    Bounced(_) => log.warn("the reminder for {id} bounced")
  }
}
# endregion job

# region main
## Issues one invoice from the command line, for example
## \`eye run -- inv-1 ann@example.com\`.
pub fn main(args: List(String)) -> Result((), String) uses {entity Invoice, start remind} {
  let (Some(id), Some(email)) = (args[1], args[2]) else {
    return Err("usage: eye run -- INVOICE EMAIL")
  }
  Invoice(id).issue(email)
  Ok(())
}
# endregion main

# region test
test "a paid invoice gets no reminder" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  with clock(2026-10-12 09:00 in "UTC")
  Invoice("inv-1").issue("ann@example.com")
  advance(2 days)
  Invoice("inv-1").pay()
  advance(1 day)
  expect(sent.calls(mail.send) == [])
}

test "an unpaid invoice gets one reminder, three days later" {
  let sent = fakes(mail, from: "recordings/")
  with sent
  with clock(2026-10-12 09:00 in "UTC")
  Invoice("inv-2").issue("bob@example.com")
  Invoice("inv-2").issue("bob@example.com")  # same key: no second reminder
  advance(71 h)
  expect(sent.calls(mail.send) == [])
  advance(1 h)
  expect(sent.calls(mail.send) == [{ to: "bob@example.com", text: "Invoice inv-2 is still open.", key: "remind-inv-2" }])
}
# endregion test
`,yb=`{
  "contract": "Mail",
  "exchanges": [
    { "op": "send", "key": "remind-inv-1", "result": { "kind": "Sent", "id": "m-1" } },
    { "op": "send", "key": "remind-inv-2", "result": { "kind": "Sent", "id": "m-2" } }
  ]
}
`,bb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,wb=`# Tutorial 5: a job that waits. A hold on stock lasts 15 minutes, unless the
# order is paid for first.

type StockError = | OutOfStock(left: Int)

# region shelf
entity Shelf(item: String) {
  state count: Int = 0
  state held: Map(String, Int) = {}

  pub fn restock(n: Int) { count += n }

  pub fn hold(order: String, n: Int) -> Result((), StockError) uses {start expire} {
    if n > count { return Err(OutOfStock(left: count)) }
    count -= n
    held = held |> insert(order, n)
    expire.start(item, order, key: "expire-{item}-{order}")
    Ok(())
  }

  pub fn sell(order: String) { held = held |> remove(order) }

  pub fn release(order: String) -> Bool {
    let Some(n) = held[order] else { return false }
    count += n
    held = held |> remove(order)
    true
  }
}
# endregion shelf

# region job
job expire(item: String, order: String) {
  sleep!(15 min)
  if Shelf(item).release!(order) {
    log.info("released {order}'s hold on {item} at {now!()}")
  }
}
# endregion job

# region test
test "a hold ends after 15 minutes unless the order is paid" {
  with clock(2026-10-10 12:00 in "Europe/Dublin")
  let apples = Shelf("apples")
  apples.restock(5)
  expect(apples.hold("order-1", 2) == Ok(()))
  expect(apples.hold("order-2", 1) == Ok(()))
  expect(apples.count == 2)

  advance(10 min)
  apples.sell("order-2")
  expect(apples.count == 2)

  advance(5 min)
  expect(apples.count == 4)
  expect(apples.held == {})
}
# endregion test

# region main
pub fn main() uses {entity Shelf, start expire} {
  let apples = Shelf("apples")
  if apples.read().version == 0 {
    apples.restock(5)
    let _ = apples.hold("order-1", 2)
  }
}
# endregion main
`,vb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,kb=`# Tutorial 4: an entity. A shop's shelves, one entity per item.

# region error
type StockError = | OutOfStock(left: Int)
# endregion error

# region shelf
entity Shelf(item: String) {
  state count: Int = 0
  state held: Map(String, Int) = {}

  pub fn restock(n: Int) { count += n }

  pub fn hold(order: String, n: Int) -> Result((), StockError) {
    if n > count { return Err(OutOfStock(left: count)) }
    count -= n
    held = held |> insert(order, n)
    Ok(())
  }

  pub fn release(order: String) {
    let n = held[order] ?? 0
    count += n
    held = held |> remove(order)
  }
}
# endregion shelf

# region test-shelf
test "an entity keeps its state between calls" {
  let apples = Shelf("apples")
  expect(apples.count == 0)
  apples.restock(5)
  expect(apples.hold("order-1", 3) == Ok(()))
  expect(apples.count == 2)
  expect(apples.hold("order-2", 3) == Err(OutOfStock(left: 2)))
  apples.release("order-1")
  expect(apples.count == 5)
}
# endregion test-shelf

# region identity
test "the same id is the same entity" {
  Shelf("pears").restock(4)
  expect(Shelf("pears").count == 4)
  expect(Shelf("plums").count == 0)
}
# endregion identity

# region refs
type Line = { shelf: Ref(Shelf), n: Int }

fn holdAll(order: String, lines: List(Line)) -> List(String) {
  var short = []
  for line in lines {
    match line.shelf.hold(order, line.n) {
      Ok(_) => ()
      Err(OutOfStock(_)) => short += [line.shelf.item]
    }
  }
  short
}
# endregion refs

# region test-refs
test "a Ref names an entity, and can be stored" {
  Shelf("milk").restock(2)
  Shelf("bread").restock(1)
  let lines: List(Line) = [
    { shelf: Shelf("milk"), n: 1 },
    { shelf: Shelf("bread"), n: 2 },
  ]
  expect(holdAll("order-7", lines) == ["bread"])
  expect(Shelf("milk").count == 1)
  let snapshot = Shelf("milk").read()
  expect(snapshot.held == { "order-7": 1 })
  expect(snapshot.version == 2)
}
# endregion test-refs

# region all
test "Shelf.all lists every shelf" {
  Shelf("tea").restock(3)
  Shelf("jam").restock(0)
  let stocked = Shelf.all |> filter(fn s: s.count > 0) |> map(fn s: s.item)
  expect(stocked == ["tea"])
}
# endregion all
`,xb=`# The access service: who is in which group. It has no expiry of its own, so
# the approvals program takes access away when it runs out.

import notify.{UserId}

pub type GrantId = | GrantId(String)

pub type Grant = { id: GrantId, person: UserId, group: String }

pub type Membership =
  | Added(Grant)
  | NoSuchGroup

pub type Removal =
  | Removed
  | AlreadyGone

# region contract
contract Access {
  ## Adds a person to a group. The service remembers what each key did.
  @http("POST /v1/grants")
  @key(required)
  @reconcile(grantFor)
  command grant(person: UserId, group: String) -> Membership

  ## What the grant with this key did, or None if it never arrived.
  @http("GET /v1/grants/by-key/:key")
  query grantFor(key: Key) -> Option(Membership)

  ## Takes a person out of a group again.
  @http("POST /v1/grants/:grant/revoke")
  @key(required)
  @reconcile(revoke)
  command revoke(grant: GrantId) -> Removal
}
# endregion contract
`,Sb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,Cb=`# Access approvals. Someone asks for temporary access to a group; a model
# summarises the request for the approver; the approver answers with a button
# in their chat; the access is granted, and taken away again when its time is
# up. On weekday mornings the approver hears what is still waiting.

import ai
import access.{Grant}
import notify.{Choice, UserId}

# region setup
## Who answers access requests.
let approver = UserId("operator")

## Schedules and the times in messages are Irish.
let dublin: Zone = "Europe/Dublin"
# endregion setup

# region details
pub type RequestId = | RequestId(String)

## What someone asks for.
pub type Details = {
  person: UserId,
  ## A group in the access service, such as "prod-db".
  group: String,
  reason: String,
  lasting: Duration,
}
# endregion details

# region policy
## The groups that can be asked for, and the longest each can be held.
let groups: Map(String, Duration) = { "prod-db": 4 h, "billing": 8 h, "vpn-admin": 1 h }

pub type Inadmissible =
  | UnknownGroup(String)
  | TooLong(most: Duration)

## Checks a request against the rules before anyone is asked.
pub fn admissible(d: Details) -> Result((), Inadmissible) {
  let Some(most) = groups[d.group] else { return Err(UnknownGroup(d.group)) }
  if d.lasting > most { return Err(TooLong(most:)) }
  Ok(())
}
# endregion policy

# region request
pub type Status =
  | New
  | Waiting
  | Denied(why: String)
  | Unanswered
  | Active(grant: Grant, until: Instant)
  | Ended

entity Request(id: RequestId) {
  state details: Option(Details) = None
  state status: Status = New
  state summary: String = ""
  state reminders: Int = 0

  ## Takes a request in and starts its review, in one transaction. Sending
  ## the same request again changes nothing and starts nothing.
  pub fn submit(d: Details) uses {start review} {
    if status != New { return }
    details = Some(d)
    match admissible(d) {
      Ok(_) => {
        status = Waiting
        review.start(id, d, key: "review-{id}")
      }
      Err(why) => status = Denied(why: "{why}")
    }
  }

  pub fn summarise(text: String) { summary = text }

  pub fn remind() { reminders += 1 }

  pub fn activate(g: Grant, until: Instant) { status = Active(grant: g, until:) }

  pub fn close(s: Status) { status = s }
}
# endregion request

# region active
## Every request whose access has been granted and not yet taken away.
view active = Request.all
  |> filter(fn r: match r.status {
    Active(..) => true
    _ => false
  })
# endregion active

# region summary
## What the approver reads about a request, written by a model.
type Summary = {
  ## One sentence, at most 20 words: who wants what, and why.
  text: String,
  risk: Risk,
}

type Risk =
  | Low
  | Medium
  ## Production data, money or admin rights.
  | High(reason: String)

let instructions = """
  You help an operator review requests for temporary access to a group.
  Summarise the request in one sentence and rate its risk. Production data,
  money and admin rights are never Low. Use only what the request says.
  """

## Asks a model to summarise a request. If the model fails, the approver
## gets the request without a summary rather than no request at all.
fn summarise(d: Details) -> Summary {
  let reply: Result(Summary, ai.AiError) = ai.generate!(
    model: "claude-sonnet-5-5",
    system: instructions,
    prompt: d |> toJson,
  )
  match reply {
    Ok(summary) => summary
    Err(e) => { text: "No summary: the model failed ({e}).", risk: Medium }
  }
}

fn describe(r: Risk) -> String = match r {
  Low => "low risk"
  Medium => "medium risk"
  High(reason) => "HIGH RISK: {reason}"
}
# endregion summary

# region review
let allow: Choice = { id: "allow", label: "Allow" }
let deny: Choice = { id: "deny", label: "Deny" }

type ReviewError =
  | NotAsked(reason: String)
  | NotGranted(group: String)

## Reviews one request: a summary from the model, a question to the approver,
## then the grant. The approver gets one question at a time, in the order the
## requests came in.
@concurrency(1)
@unwitnessed("Notify declines to ask only when the approver has no device, which no recording has")
job review(id: RequestId, d: Details) -> Result(Status, ReviewError) {
  let summary = summarise!(d)
  Request(id).summarise!(summary.text)
  let asked = notify.ask!(
    to: approver,
    text: "{d.person} asks for {d.group} for {d.lasting} ({describe(summary.risk)}). {summary.text}",
    choices: [allow, deny],
    within: 3 days,
    key: "review-{id}",
  ).reconcile!()
  let question = match asked {
    Asking(q) => q
    NotAsked(reason) => return Err(NotAsked(reason))
  }
  let Some(answer) = next!(notify.answered, where: fn a: a.question == question, within: 3 days) else {
    Request(id).close!(Unanswered)
    tell!(d.person, "Nobody answered your request for {d.group}. Ask again if you still need it.", key: "unanswered-{id}")
    return Ok(Unanswered)
  }
  if answer.choice == deny.id {
    let denied = Denied(why: "{answer.by} said no")
    Request(id).close!(denied)
    tell!(d.person, "Your request for {d.group} was denied.", key: "denied-{id}")
    return Ok(denied)
  }
  let until = now!() + d.lasting
  match access.grant!(d.person, d.group, key: "grant-{id}").reconcile!() {
    Added(grant) => {
      Request(id).activate!(grant, until)
      tell!(d.person, "You are in {d.group} until {until.time(in: dublin)}.", key: "granted-{id}")
      Ok(Active(grant:, until:))
    }
    NoSuchGroup => Err(NotGranted(d.group))
  }
}
# endregion review

# region tell
## Tells a person how their request went. Notify keeps what each key did, so
## a lost reply is settled by sending again with the same key.
@unwitnessed("Notify refuses a send only for a person it doesn't know, which no recording has")
pub fn tell(person: UserId, text: String, key: Key) uses {wait, net Notify.send} {
  match notify.send!(to: person, text:, key:).reconcile!() {
    Accepted(_) => ()
    Refused(error) => log.warn("Notify refused {key}: {error}")
  }
}
# endregion tell

# region expire
## Every request that becomes active gets one run of this job. It sleeps
## until the access runs out, then takes it away.
@on(enters(active))
job expire(r: Ref(Request)) {
  let Active(grant, until) = r.read!().status else { return }
  sleepUntil!(until)
  match access.revoke!(grant.id, key: "revoke-{grant.id}").reconcile!() {
    Removed | AlreadyGone => ()
  }
  r.close!(Ended)
  tell!(grant.person, "Your access to {grant.group} has ended.", key: "ended-{grant.id}")
}
# endregion expire

# region morning
## On weekday mornings the approver hears how many requests are waiting.
@on(weekdays(09:00, in: dublin))
job morning(day: Date) {
  let open = Request.all.read!() |> filter(fn r: r.status == Waiting)
  if open == [] { return }
  for r in open { Request(r.id).remind!() }
  tell!(approver, "{open |> size} access requests are waiting for you.", key: "waiting-{day}")
}
# endregion morning

# region main
## Submits one request from the command line, for example
## \`eye run . -- r-1 ann prod-db 2 "fix the stuck migration"\`. Running it
## again with the same id changes nothing.
pub fn main(args: List(String)) -> Result((), String) uses {entity Request, start review} {
  let usage = "usage: eye run . -- ID PERSON GROUP HOURS REASON"
  let (Some(id), Some(person), Some(group), Some(hours), Some(reason)) = (args[1], args[2], args[3], args[4], args[5]) else {
    return Err(usage)
  }
  let n = parseInt(hours).mapErr(fn _: usage)?
  Request(RequestId(id)).submit({ person: UserId(person), group, reason, lasting: 1 h * n })
  Ok(())
}
# endregion main

# ---------------------------------------------------------------- tests

# region test-data
let ann = UserId("ann")
let first = RequestId("r-1")

let fix: Details = {
  person: ann,
  group: "prod-db",
  reason: "Fix the stuck migration",
  lasting: 2 h,
}

fn answer(q: String, choice: String) {
  notify.answered.publish({ question: notify.QuestionId(q), choice, by: approver, at: now() })
}
# endregion test-data

# region test-policy
test "a request outside the rules is denied before anyone is asked" {
  expect(admissible({ fix with group: "root" }) == Err(UnknownGroup("root")))
  Request(first).submit({ fix with lasting: 12 h })
  expect(Request(first).status == Denied(why: "TooLong(most: 4 h)"))
}
# endregion test-policy

# region test-allow
test "the approver allows, and the access ends when its time is up" {
  with fakes(ai, notify, access, from: "recordings/")
  with clock(2026-10-12 10:00 in dublin)
  let r = Request(first)
  r.submit(fix)
  expect(r.status == Waiting)
  expect(r.summary == "Ann wants two hours on the production database to fix a stuck migration.")
  answer("q-1", "allow")
  expect(r.status == Active(grant: { id: access.GrantId("g-1"), person: ann, group: "prod-db" }, until: 2026-10-12 12:00 in dublin))
  advance(119 min)
  expect(r.status != Ended)
  advance(1 min)
  expect(r.status == Ended)
}
# endregion test-allow

# region test-deny
test "the approver says no" {
  with fakes(ai, notify, from: "recordings/")
  Request(first).submit(fix)
  answer("q-1", "deny")
  expect(Request(first).status == Denied(why: "operator said no"))
}
# endregion test-deny

# region test-unanswered
test "a request nobody answers for three days is closed" {
  with fakes(ai, notify, from: "recordings/")
  with clock(2026-10-16 10:00 in dublin)  # a Friday
  Request(first).submit(fix)
  advance(71 h)
  expect(Request(first).status == Waiting)
  advance(1 h)
  expect(Request(first).status == Unanswered)
}
# endregion test-unanswered

# region test-lost
test "a lost reply from the access service is settled by asking, not by granting again" {
  with fakes(ai, notify, access, from: "recordings/")
  let r = Request(RequestId("r-2"))
  r.submit({ person: UserId("bob"), group: "billing", reason: "Check an invoice", lasting: 1 h })
  answer("q-2", "allow")
  let Active(grant, ..) = r.status else { unreachable("the request should be active") }
  expect(grant.id == access.GrantId("g-2"))
}
# endregion test-lost

# region test-model-down
test "a model that fails costs the summary, not the request" {
  with fakes(notify, from: "recordings/")
  with fakes(ai, from: "recordings/model-down/")
  Request(first).submit(fix)
  expect(Request(first).status == Waiting)
  expect(Request(first).summary |> startsWith("No summary"))
}
# endregion test-model-down

# region test-one-at-a-time
test "the approver gets one question at a time, in order" {
  with fakes(ai, notify, from: "recordings/")
  let second = Request(RequestId("r-2"))
  Request(first).submit(fix)
  second.submit({ person: UserId("bob"), group: "billing", reason: "Check an invoice", lasting: 1 h })
  expect(second.summary == "")
  answer("q-1", "deny")
  expect(second.summary == "Bob wants an hour of billing access to check an invoice.")
}
# endregion test-one-at-a-time

# region test-morning
test "on weekday mornings the approver hears what is waiting" {
  with fakes(ai, notify, from: "recordings/")
  with clock(2026-10-16 08:00 in dublin)  # a Friday
  Request(first).submit(fix)
  advance(1 h)
  expect(Request(first).reminders == 1)
  advance(2 days)  # Saturday and Sunday bring no reminder
  expect(Request(first).reminders == 1)
}
# endregion test-morning
`,Ab=`# Notify sends messages to people in their chat, and asks them questions with
# buttons. This contract is its interface; the approvals program calls it.

pub type UserId = | UserId(String)

pub type MessageId = | MessageId(String)

## What Notify did with a message.
pub type Delivery =
  ## The outbox has it. A repeated key gives the first answer again.
  | Accepted(id: MessageId)
  ## A bad request, or a person Notify doesn't know.
  | Refused(error: String)

## A button under a message: \`id\` comes back in the answer.
pub type Choice = { id: String, label: String }

pub type QuestionId = | QuestionId(String)

pub type Asked =
  | Asking(QuestionId)
  | NotAsked(reason: String)

## The first press on a question's buttons. Later presses change nothing.
pub type Answered = { question: QuestionId, choice: String, by: UserId, at: Instant }

# region contract
@auth(bearer: "notify-token")
contract Notify {
  ## Sends a message to one person. Notify keeps what each key did, so sending
  ## again with the same key is how a lost reply is settled.
  @http("POST /v1/send", status: { 200: Accepted, 400: Refused })
  @key(required)
  @reconcile(send)
  command send(to: UserId, text: String) -> Delivery

  ## Asks one person, with 2 to 4 buttons. The question closes after
  ## \`within\`; an answer comes as an \`answered\` event.
  @http("POST /v1/ask")
  @key(required)
  @reconcile(ask)
  command ask(to: UserId, text: String, choices: List(Choice), within: Duration) -> Asked

  @key(question)
  event answered: Answered on "notify.answers"
}
# endregion contract
`,Tb=`{
  "contract": "Access",
  "exchanges": [
    {
      "op": "grant",
      "key": "grant-r-1",
      "result": { "kind": "Added", "value": { "id": "g-1", "person": "ann", "group": "prod-db" } }
    },
    { "op": "grant", "key": "grant-r-2", "result": { "kind": "Uncertain", "key": "grant-r-2" } },
    {
      "op": "grantFor",
      "key": "grant-r-2",
      "result": { "kind": "Added", "value": { "id": "g-2", "person": "bob", "group": "billing" } }
    },
    { "op": "revoke", "key": "revoke-g-1", "result": "Removed" }
  ]
}
`,Mb=`{
  "contract": "ai",
  "exchanges": [
    {
      "op": "generate",
      "result": {
        "text": "Ann wants two hours on the production database to fix a stuck migration.",
        "risk": { "kind": "High", "reason": "write access to production data" }
      }
    },
    {
      "op": "generate",
      "result": { "text": "Bob wants an hour of billing access to check an invoice.", "risk": { "kind": "Medium" } }
    }
  ]
}
`,Ob=`{
  "contract": "ai",
  "exchanges": [
    { "op": "generate", "result": { "summary": "Ann wants the database." } },
    { "op": "generate", "result": { "summary": "Ann wants the database." } }
  ]
}
`,Ib=`{
  "contract": "Notify",
  "exchanges": [
    { "op": "ask", "key": "review-r-1", "result": { "kind": "Asking", "value": "q-1" } },
    { "op": "ask", "key": "review-r-2", "result": { "kind": "Asking", "value": "q-2" } },
    { "op": "send", "key": "granted-r-1", "result": { "kind": "Accepted", "id": "m-1" } },
    { "op": "send", "key": "granted-r-2", "result": { "kind": "Accepted", "id": "m-2" } },
    { "op": "send", "key": "ended-g-1", "result": { "kind": "Accepted", "id": "m-3" } },
    { "op": "send", "key": "denied-r-1", "result": { "kind": "Accepted", "id": "m-4" } },
    { "op": "send", "key": "unanswered-r-1", "result": { "kind": "Accepted", "id": "m-5" } },
    { "op": "send", "key": "waiting-2026-10-16", "result": { "kind": "Accepted", "id": "m-6" } },
    { "op": "send", "key": "waiting-2026-10-19", "result": { "kind": "Accepted", "id": "m-7" } }
  ]
}
`,Db=`# Marks the root of an Eyelang program: imports name directories below this one.
`,Rb=`# Tutorial 3: errors. Reading an order and taking it from the stock.

# region parse-error
type ParseError =
  | Empty
  | NotANumber(String)
  | TooMany(Int)
# endregion parse-error

# region parse
fn parseQuantity(text: String) -> Result(Int, ParseError) {
  let t = text |> trim
  if t == "" { return Err(Empty) }
  let n = parseInt(t).mapErr(fn _: NotANumber(t))?
  if n > 99 then return Err(TooMany(n))
  Ok(n)
}
# endregion parse

# region test-parse
test "a Result is Ok or Err" {
  expect(parseQuantity(" 3 ") == Ok(3))
  expect(parseQuantity("") == Err(Empty))
  expect(parseQuantity("three") == Err(NotANumber("three")))
  expect(parseQuantity("500") == Err(TooMany(500)))
}
# endregion test-parse

# region match-result
fn message(text: String) -> String = match parseQuantity(text) {
  Ok(n) => "{n} it is"
  Err(Empty) => "how many?"
  Err(NotANumber(t)) => "'{t}' is not a number"
  Err(TooMany(n)) => "{n} is more than we sell at once"
}
# endregion match-result

# region test-match-result
test "match handles each error" {
  expect(message("2") == "2 it is")
  expect(message("") == "how many?")
  expect(message("lots") == "'lots' is not a number")
}
# endregion test-match-result

# region default
test "?? gives a default for a missing value" {
  let stock = { "apples": 5 }
  let apples = stock["apples"] ?? 0
  let pears = stock["pears"] ?? 0
  expect(apples == 5)
  expect(pears == 0)
}
# endregion default

# region order-error
type OrderError =
  | BadQuantity(ParseError)
  | OutOfStock(item: String, left: Int)
# endregion order-error

# region take
fn take(mut stock: Map(String, Int), item: String, text: String) -> Result(Int, OrderError) {
  let n = parseQuantity(text)?
  let left = stock[item] ?? 0
  if n > left { return Err(OutOfStock(item:, left:)) }
  stock = stock |> insert(item, left - n)
  Ok(n)
}
# endregion take

# region test-take
test "? passes the error on, and mut shows what may change" {
  var stock = { "apples": 5, "pears": 1 }
  expect(take(mut stock, "apples", "3") == Ok(3))
  expect(stock["apples"] == Some(2))
  expect(take(mut stock, "pears", "2") == Err(OutOfStock(item: "pears", left: 1)))
  expect(take(mut stock, "pears", "two") == Err(BadQuantity(NotANumber("two"))))
  expect(stock["pears"] == Some(1))
}
# endregion test-take

# region option-question
fn firstWord(text: String) -> Option(String) {
  let word = text |> split(" ") |> find(fn w: w != "")?
  Some(word |> upper)
}

test "? works on Option too" {
  expect(firstWord("  hello there") == Some("HELLO"))
  expect(firstWord("   ") == None)
}
# endregion option-question
`,Eb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,Lb=`# Says hello to each name it is given, or to the world.

fn greeting(name: String) -> String = "Hello, {name}!"

pub fn main(args: List(String)) uses {stdio} {
  let names = args |> drop(1)
  if names == [] {
    print(greeting("world"))
  }
  for name in names { print(greeting(name)) }
}

test "greeting names who it greets" {
  expect(greeting("Ann") == "Hello, Ann!")
}
`,Bb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,_b=`# Tutorial 2: types and matching. Tasks on a to-do list.

# region record
type Task = { title: String, done: Bool, priority: Priority }
# endregion record

# region variant
type Priority = Low | Normal | High

type Status =
  | Open
  | Snoozed(until: Date)
  | Done(on: Date)
# endregion variant

# region record-values
test "records are values" {
  let task: Task = { title: "Buy milk", done: false, priority: Normal }
  let finished = { task with done: true }
  expect(task.done == false)
  expect(finished.done == true)
  expect(finished.title == "Buy milk")
  expect(task != finished)
}
# endregion record-values

# region match
fn describe(s: Status) -> String = match s {
  Open => "open"
  Snoozed(until) => "snoozed until {until}"
  Done(on) => "done on {on}"
}
# endregion match

# region test-match
test "match takes a variant apart" {
  expect(describe(Open) == "open")
  expect(describe(Snoozed(until: 2026-10-12)) == "snoozed until 2026-10-12")
  expect(describe(Done(on: 2026-10-10)) == "done on 2026-10-10")
}
# endregion test-match

# region guards
fn urgent(t: Task) -> Bool = match t.priority {
  High if !t.done => true
  _ => false
}

fn label(p: Priority) -> String = match p {
  Low | Normal => "-"
  High => "!"
}
# endregion guards

# region test-guards
test "guards, alternatives and _" {
  let t: Task = { title: "Pay rent", done: false, priority: High }
  expect(urgent(t))
  expect(!urgent({ t with done: true }))
  expect(label(Low) == "-" && label(High) == "!")
  expect(Low < High)
}
# endregion test-guards

# region option
fn findTask(tasks: List(Task), title: String) -> Option(Task) =
  tasks |> find(fn t: t.title == title)

fn priorityOf(tasks: List(Task), title: String) -> String =
  match findTask(tasks, title) {
    Some(t) => label(t.priority)
    None => "no such task"
  }
# endregion option

# region test-option
test "Option says a value may be missing" {
  let tasks: List(Task) = [
    { title: "Buy milk", done: false, priority: Normal },
    { title: "Pay rent", done: false, priority: High },
  ]
  expect(priorityOf(tasks, "Pay rent") == "!")
  expect(priorityOf(tasks, "Walk dog") == "no such task")
  let title = findTask(tasks, "Buy milk") |> map(fn t: t.title)
  expect(title == Some("Buy milk"))
  let missing = findTask(tasks, "Walk dog") |> map(fn t: t.title) ?? "none"
  expect(missing == "none")
}
# endregion test-option

# region impl
impl Task {
  fn new(title: String) -> Task = { title, done: false, priority: Normal }

  fn line(self) -> String {
    let mark = if self.done then "x" else " "
    "[{mark}] {label(self.priority)} {self.title}"
  }

  fn finish(self) -> Task = { self with done: true }
}
# endregion impl

# region test-impl
test "methods" {
  let t = Task.new("Buy milk")
  expect(t.line() == "[ ] - Buy milk")
  expect(t.finish().line() == "[x] - Buy milk")
}
# endregion test-impl
`,Pb=`# Marks the root of an Eyelang program: imports name directories below this one.
`,Nb=`# Tutorial 1: values and functions. A small shopping list, priced in cents.

# region functions
fn double(n: Int) -> Int = n * 2

fn price(cents: Int) -> String {
  let euros = cents / 100
  let rest = cents % 100
  if rest < 10 then "€{euros}.0{rest}" else "€{euros}.{rest}"
}
# endregion functions

# region test-functions
test "functions" {
  expect(double(21) == 42)
  expect(price(250) == "€2.50")
  expect(price(1205) == "€12.05")
}
# endregion test-functions

# region lets
test "let names a value" {
  let apples = 3
  let each = 40
  let total = apples * each
  expect(total == 120)
  expect("{apples} apples cost {price(total)}" == "3 apples cost €1.20")
}
# endregion lets

# region lists
test "lists" {
  let basket = [250, 120, 80]
  expect(basket[0] == Some(250))
  expect(basket[5] == None)
  expect(basket + [99] == [250, 120, 80, 99])
  expect(basket |> sort == [80, 120, 250])
  expect(basket == [250, 120, 80])
  expect(basket |> size == 3)
}
# endregion lists

# region pipes
fn total(basket: List(Int)) -> Int = basket |> sum

fn bargains(basket: List(Int), under: Int) -> List(String) = basket
  |> filter(fn cents: cents < under)
  |> sort
  |> map(price)
# endregion pipes

# region test-pipes
test "pipes pass the value on" {
  let basket = [250, 120, 80]
  expect(total(basket) == 450)
  expect(bargains(basket, under: 200) == ["€0.80", "€1.20"])
  expect(basket |> map(double) == [500, 240, 160])
}
# endregion test-pipes

# region closures
fn discount(percent: Int) = fn cents: cents - cents * percent / 100

test "closures keep what they capture" {
  let tenOff = discount(10)
  expect(tenOff(250) == 225)
  expect([250, 120] |> map(tenOff) == [225, 108])
}
# endregion closures

# region var-for
fn countOver(basket: List(Int), limit: Int) -> Int {
  var count = 0
  for cents in basket {
    if cents > limit then count += 1
  }
  count
}

test "var and for" {
  expect(countOver([250, 120, 80], limit: 100) == 2)
  var sum = 0
  for i in 1..=4 { sum += i }
  expect(sum == 10)
  var basket = [250, 120, 80]
  basket[1] = 99
  basket[2] += 1
  expect(basket == [250, 99, 81])
}
# endregion var-for
`,Wb=Object.assign({"../../../examples/daily-report/chat/chat.eye":sy,"../../../examples/daily-report/eye.toml":ry,"../../../examples/daily-report/main.eye":oy,"../../../examples/daily-report/recordings/chat.json":ay,"../../../examples/index.md":ly,"../../../examples/log-summary/eye.toml":hy,"../../../examples/log-summary/main.eye":cy,"../../../examples/refund-approval/eye.toml":dy,"../../../examples/refund-approval/mail/mail.eye":uy,"../../../examples/refund-approval/main.eye":fy,"../../../examples/refund-approval/payments/payments.eye":py,"../../../examples/refund-approval/recordings/mail.json":my,"../../../examples/refund-approval/recordings/payments.json":gy,"../../../examples/shop-and-bank/bank/bank.eye":yy,"../../../examples/shop-and-bank/bank/eye.toml":by,"../../../examples/shop-and-bank/eye.toml":wy,"../../../examples/shop-and-bank/main.eye":vy,"../../../examples/shop-and-bank/payments/payments.eye":ky,"../../../examples/shop-and-bank/recordings/payments.json":xy,"../../../examples/trial-ending/eye.toml":Sy,"../../../examples/trial-ending/mail/mail.eye":Cy,"../../../examples/trial-ending/main.eye":Ay,"../../../examples/trial-ending/recordings/mail.json":Ty,"../../../examples/uptime-monitor/chat/chat.eye":My,"../../../examples/uptime-monitor/eye.toml":Oy,"../../../examples/uptime-monitor/main.eye":Iy,"../../../examples/uptime-monitor/recordings/chat.json":Dy,"../../../examples/uptime-monitor/recordings/down/http.json":Ry,"../../../examples/uptime-monitor/recordings/up/http.json":Ey,"../../../examples/webhook-delivery/endpoint/endpoint.eye":Ly,"../../../examples/webhook-delivery/eye.toml":By,"../../../examples/webhook-delivery/main.eye":_y,"../../../examples/webhook-delivery/recordings/down/endpoint.json":Py,"../../../examples/webhook-delivery/recordings/endpoint.json":Ny,"../../../guide/code/ai-calls/eye.toml":Wy,"../../../guide/code/ai-calls/main.eye":qy,"../../../guide/code/ai-calls/recordings/ai.json":Hy,"../../../guide/code/ai-calls/recordings/garbled/ai.json":Fy,"../../../guide/code/contracts/eye.toml":Vy,"../../../guide/code/contracts/invoices/invoices.eye":zy,"../../../guide/code/contracts/main.eye":jy,"../../../guide/code/contracts/payments/payments.eye":Uy,"../../../guide/code/contracts/recordings/lost-text/sms.json":$y,"../../../guide/code/contracts/recordings/payments.json":Ky,"../../../guide/code/contracts/recordings/sms.json":Gy,"../../../guide/code/contracts/sms/sms.eye":Yy,"../../../guide/code/doc-comments/eye.toml":Jy,"../../../guide/code/doc-comments/shipping.eye":Xy,"../../../guide/code/effects/chat/chat.eye":Qy,"../../../guide/code/effects/eye.toml":Zy,"../../../guide/code/effects/main.eye":eb,"../../../guide/code/effects/recordings/chat.json":tb,"../../../guide/code/entities/eye.toml":nb,"../../../guide/code/entities/main.eye":ib,"../../../guide/code/jobs-and-waiting/eye.toml":sb,"../../../guide/code/jobs-and-waiting/main.eye":rb,"../../../guide/code/testing/eye.toml":ob,"../../../guide/code/testing/format/format.eye":ab,"../../../guide/code/testing/main.eye":lb,"../../../guide/code/testing/notify/notify.eye":hb,"../../../guide/code/testing/recordings/notify.json":cb,"../../../guide/code/views-events-schedules/eye.toml":db,"../../../guide/code/views-events-schedules/main.eye":ub,"../../../guide/code/views-events-schedules/monitor/monitor.eye":fb,"../../../quickstart/code/remind/eye.toml":pb,"../../../quickstart/code/remind/mail/mail.eye":mb,"../../../quickstart/code/remind/main.eye":gb,"../../../quickstart/code/remind/recordings/mail.json":yb,"../../../tutorial/code/a-job-that-waits/eye.toml":bb,"../../../tutorial/code/a-job-that-waits/main.eye":wb,"../../../tutorial/code/an-entity/eye.toml":vb,"../../../tutorial/code/an-entity/main.eye":kb,"../../../tutorial/code/approvals/access/access.eye":xb,"../../../tutorial/code/approvals/eye.toml":Sb,"../../../tutorial/code/approvals/main.eye":Cb,"../../../tutorial/code/approvals/notify/notify.eye":Ab,"../../../tutorial/code/approvals/recordings/access.json":Tb,"../../../tutorial/code/approvals/recordings/ai.json":Mb,"../../../tutorial/code/approvals/recordings/model-down/ai.json":Ob,"../../../tutorial/code/approvals/recordings/notify.json":Ib,"../../../tutorial/code/errors/eye.toml":Db,"../../../tutorial/code/errors/main.eye":Rb,"../../../tutorial/code/hello/eye.toml":Eb,"../../../tutorial/code/hello/main.eye":Lb,"../../../tutorial/code/types-and-matching/eye.toml":Bb,"../../../tutorial/code/types-and-matching/main.eye":_b,"../../../tutorial/code/values-and-functions/eye.toml":Pb,"../../../tutorial/code/values-and-functions/main.eye":Nb}),ul={quickstart:"Start",examples:"Examples",tutorial:"Tutorial",guide:"Guide"},fl={"quickstart/remind":"Quickstart: a reminder","examples/trial-ending":"Trial ending: an email three days before","examples/webhook-delivery":"Webhook delivery: keys, lost replies and backoff","examples/uptime-monitor":"Uptime monitor: probe every minute, alert once","examples/daily-report":"Daily report: 08:00 in New York, every weekday","examples/refund-approval":"Refund approval: a person says yes first","examples/log-summary":"Log summary: the plain language","examples/shop-and-bank":"A shop and its bank: two programs, one contract","tutorial/hello":"Hello","tutorial/values-and-functions":"1. Values and functions","tutorial/types-and-matching":"2. Types and matching","tutorial/errors":"3. Errors","tutorial/an-entity":"4. An entity","tutorial/a-job-that-waits":"5. A job that waits","tutorial/approvals":"All together: approvals","guide/entities":"Entities","guide/jobs-and-waiting":"Jobs and waiting","guide/views-events-schedules":"Views, events and schedules","guide/testing":"Testing with time and recordings","guide/contracts":"Talking to other services","guide/effects":"Effects","guide/ai-calls":"AI calls","guide/doc-comments":"Doc comments"},qb={"quickstart/remind":"inv-1 ann@example.com","examples/trial-ending":"ann ann@example.com"};function Hb(){const i=new Map;for(const[s,r]of Object.entries(Wb)){const o=/\/(?:(quickstart|tutorial|guide)\/code|(examples))\/([^/]+)\/(.+)$/.exec(s);if(!o)continue;const[,a,l,h,c]=o,d=a??l,u=`${d}/${h}`;let f=i.get(u);f||(f={id:u,group:ul[d],title:fl[u]??h,files:{},args:qb[u]},i.set(u,f)),f.files[c]=r}const e=Object.values(ul),t=Object.keys(fl),n=s=>t.includes(s)?t.indexOf(s):t.length;return[...i.values()].sort((s,r)=>e.indexOf(s.group)-e.indexOf(r.group)||n(s.id)-n(r.id)||s.id.localeCompare(r.id))}const od=Hb(),Wn={id:"blank",group:"",title:"Empty program",files:{"eye.toml":"","main.eye":`pub fn main(args: List(String)) uses {stdio} {
  print("Hello!")
}
`}},Fb="quickstart/remind";function Mn(i){return i===Wn.id?Wn:od.find(e=>e.id===i)}const ad="eyelang-playground";function Vb(){try{const i=localStorage.getItem(ad);return i?JSON.parse(i):void 0}catch{return}}function zb(i){try{localStorage.setItem(ad,JSON.stringify(i))}catch{}}const jb=i=>{let e="";for(const t of i)e+=String.fromCharCode(t);return btoa(e).replace(/\+/g,"-").replace(/\//g,"_").replace(/=+$/,"")},Ub=i=>{const e=atob(i.replace(/-/g,"+").replace(/_/g,"/"));return Uint8Array.from(e,t=>t.charCodeAt(0))};async function ld(i,e){const t=new Response(new Blob([i]).stream().pipeThrough(e));return new Uint8Array(await t.arrayBuffer())}async function $b(i){const e=new TextEncoder().encode(JSON.stringify(i));return jb(await ld(e,new CompressionStream("deflate-raw")))}async function Kb(i){const e=await ld(Ub(i),new DecompressionStream("deflate-raw")),t=JSON.parse(new TextDecoder().decode(e));if(typeof t!="object"||t===null)throw new Error("not a program");for(const[n,s]of Object.entries(t))if(typeof s!="string")throw new Error(`${n} isn't text`);return t}async function Gb(i){const e=new URLSearchParams(i.replace(/^#/,""));if(e.get("code"))try{return{files:await Kb(e.get("code"))}}catch{return}if(e.get("example"))return{example:e.get("example")}}const Yb={class:"pg"},Jb={class:"pg-bar"},Xb=["value"],Qb=["label"],Zb=["value"],ew=["value"],tw=["disabled","title"],nw=["disabled","title"],iw=["disabled","title"],sw={class:"pg-args"},rw={class:"pg-main"},ow={class:"pg-editor"},aw={class:"pg-tabs",role:"tablist"},lw=["aria-selected","onClick","onDblclick"],hw=["aria-label","onClick"],cw={class:"pg-side"},dw={class:"pg-tabs",role:"tablist"},uw={key:0,class:"pg-count"},fw={class:"pg-output","aria-live":"polite"},pw={class:"pg-problems"},mw={key:0,class:"pg-none"},gw=["onClick"],yw={key:0,class:"pg-where"},kw=cd({__name:"Playground",setup(i){const e=Ze(),t=Lo(),n=Lo(),s=new Map,r=bd({}),o=Ze("main.eye"),a=Ze(""),l=Ze(""),h=Ze({state:"loading"}),c=Ze([]),d=Ze([]),u=Ze("output"),f=Ze(""),p=Ze(""),m=Es(()=>Object.keys(r).sort((w,k)=>{const T=B=>B==="main.eye"?0:B.endsWith(".eye")?1:B==="eye.toml"?2:3;return T(w)-T(k)||w.localeCompare(k)})),g=Es(()=>{var k;const w={};for(const T of od)(w[k=T.group]??(w[k]=[])).push(T);return w}),y=Es(()=>{const w=h.value;if(w.state==="failed")return`The engine didn't start: ${w.error}`;if(w.state==="ready")return"";const k=(T=0)=>(T/1e6).toFixed(1);return w.loaded?`Loading the engine… ${k(w.loaded)}${w.total?` of ${k(w.total)}`:""} MB`:"Loading the engine…"}),x={onChange(w,k){var T;r[w]=k,(T=t.value)==null||T.setFiles(r),L()},onRun:()=>E(),onTest:()=>$(),onFormat:()=>K(),onJump:w=>q(w),onProblems(w){d.value=w}};function v(w){let k=s.get(w);return k||(k=ny(t.value,w,r[w]??"",x),s.set(w,k)),k}function I(w){!n.value||!(w in r)||(s.set(o.value,n.value.state),o.value=w,n.value.setState(v(w)),n.value.focus(),L())}function S(w,k){var B;for(const se of Object.keys(r))delete r[se];Object.assign(r,w),s.clear(),(B=t.value)==null||B.setFiles(r),d.value=[];const T="main.eye"in w?"main.eye":m.value[0];o.value=T??"main.eye",n.value&&T&&n.value.setState(v(T)),L()}function C(w){const k=Mn(w);if(!k)return;l.value=w,a.value=k.args??"",S(k.files),c.value=[{kind:"info",text:`${k.title}: Run (${ge}+Enter) runs main; Test (${ge}+Shift+Enter) runs the tests.
`}];const T=Object.keys(k.files).filter(B=>B.endsWith("/eye.toml")).map(B=>B.slice(0,-9));for(const B of T)F("info",`${B}/ has an eye.toml of its own, so it is a second program: Run starts it first, and the calls to the contract it serves reach it here, in the page.
`);location.hash&&history.replaceState(null,"",location.pathname+location.search)}let A;function L(){clearTimeout(A),A=setTimeout(()=>zb({files:{...r},active:o.value,args:a.value,example:l.value}),300)}dd(a,L);function q(w){w.path!==o.value&&I(w.path);const k=n.value;if(!k)return;const T=as(k.state.doc,w.range.start),B=as(k.state.doc,w.range.end);k.dispatch({selection:{anchor:T,head:B},effects:R.scrollIntoView(T,{y:"center"})}),k.focus()}function F(w,k){const T=c.value[c.value.length-1];T&&T.kind===w?T.text+=k:c.value.push({kind:w,text:k}),wd(()=>{const B=document.querySelector(".pg-output");B&&(B.scrollTop=B.scrollHeight)})}function W(w){const k=[];for(const T of w.matchAll(/"([^"]*)"|'([^']*)'|(\S+)/g))k.push(T[1]??T[2]??T[3]);return k}async function E(){if(!t.value||f.value)return;f.value="run",u.value="output",c.value=[];const w=performance.now();try{const k={stdout:"stdout",stderr:"stderr",service:"service"},T=/^eye run: (started |\S+ answers from the recordings |the virtual clock stopped )/,B=/^\S+\.\S+( with key "[^"]*")? at \d{4}-\d\d-\d\d \d\d:\d\d → .*, from \S+\.json\n$/,se=await t.value.run(W(a.value),(Os,Is)=>{let Ds=k[Os]??"stderr";Os==="stderr"&&T.test(Is)?Ds="info":Os==="stderr"&&B.test(Is)&&(Ds="service"),F(Ds,Is)});se.diagnostics.length&&(d.value=se.diagnostics);const hd=Math.round(performance.now()-w);F("info",`
${se.exit===0?"exit 0":`exit ${se.exit}`} · ${hd} ms
`)}catch(k){F("error",`
${k.message}
`)}finally{f.value=""}}async function $(){if(!(!t.value||f.value)){f.value="test",u.value="output",c.value=[];try{const w=await t.value.test("");if(w.diagnostics.length){d.value=w.diagnostics,F("stderr",w.output);return}for(const k of w.output.split(new RegExp("(?<=\\n)"))){const T=k.startsWith("ok ")?"pass":k.startsWith("FAIL")?"fail":k.startsWith("      ")?"stdout":"info";F(T,k)}}catch(w){F("error",`
${w.message}
`)}finally{f.value=""}}}function V(){var w;(w=t.value)==null||w.stop("stopped")}async function K(){const w=n.value;if(!(!t.value||!w||!o.value.endsWith(".eye")))try{const{text:k}=await t.value.format(o.value);k!==w.state.doc.toString()&&w.dispatch({changes:{from:0,to:w.state.doc.length,insert:k}})}catch(k){ee(k.message)}}function ee(w){p.value=w,setTimeout(()=>p.value===w&&(p.value=""),3e3)}async function ke(){const w=await $b({...r}),k=`${location.origin}${location.pathname}#code=${w}`;history.replaceState(null,"",k);try{await navigator.clipboard.writeText(k),ee("Link copied: it holds the whole program")}catch{ee("The link is in the address bar")}}const Ee=w=>/^([a-z][a-z0-9_]*\/)*[A-Za-z0-9_.-]+\.(eye|json|toml)$/.test(w);function re(){var k,T;const w=(k=prompt("File name, such as notify/notify.eye or recordings/notify.json"))==null?void 0:k.trim();if(w){if(!Ee(w))return ee(`${w}: a file is a .eye, .json or .toml, in lower-case directories`);if(w in r)return I(w);r[w]=w.endsWith(".eye")?"":w.endsWith(".json")?`{}
`:"",(T=t.value)==null||T.setFiles(r),I(w)}}function Me(w){var k,T;if(confirm(`Delete ${w}?`)){if(delete r[w],s.delete(w),(k=t.value)==null||k.setFiles(r),o.value===w){const B=m.value[0];B&&(o.value=B,(T=n.value)==null||T.setState(v(B)))}L()}}function Ue(w){var T,B,se;const k=(T=prompt("Rename to",w))==null?void 0:T.trim();if(!(!k||k===w)){if(!Ee(k))return ee(`${k}: a file is a .eye, .json or .toml, in lower-case directories`);if(k in r)return ee(`${k} exists already`);n.value&&o.value===w&&s.set(w,n.value.state),r[k]=n.value&&o.value===w?n.value.state.doc.toString():r[w],delete r[w],s.delete(w),(B=t.value)==null||B.setFiles(r),o.value=k,(se=n.value)==null||se.setState(v(k)),L()}}function Oe(w){w.path&&q({path:w.path,range:w.range})}const ge=typeof navigator<"u"&&/Mac|iPhone|iPad/.test(navigator.platform)?"⌘":"Ctrl";return ud(async()=>{const w=new iy(fd("/"));w.onStatus=se=>h.value=se,t.value=w;const k=await Gb(location.hash),T=Vb();let B;if(k!=null&&k.files)Object.assign(r,k.files);else if(k!=null&&k.example&&Mn(k.example))l.value=k.example,Object.assign(r,Mn(k.example).files),a.value=Mn(k.example).args??"";else if(T!=null&&T.files&&Object.keys(T.files).length)Object.assign(r,T.files),B=T.active,a.value=T.args??"",l.value=T.example??"";else{const se=Mn(Fb)??Wn;l.value=se.id,Object.assign(r,se.files),a.value=se.args??""}w.setFiles(r),o.value=B&&B in r?B:"main.eye"in r?"main.eye":m.value[0],n.value=new R({state:v(o.value),parent:e.value}),c.value=[{kind:"info",text:`Run (${ge}+Enter) runs main; Test (${ge}+Shift+Enter) runs the tests. Hover a name to see its type, ${ge}+click it to go to its definition.
`}]}),pd(()=>{var w,k;(w=n.value)==null||w.destroy(),(k=t.value)==null||k.dispose()}),(w,k)=>(ye(),be("div",Yb,[Q("div",Jb,[Q("select",{class:"pg-select",value:l.value,"aria-label":"Example",onChange:k[0]||(k[0]=T=>C(T.target.value))},[k[4]||(k[4]=Q("option",{value:"",disabled:""},"Examples…",-1)),(ye(!0),be(yn,null,bn(g.value,(T,B)=>(ye(),be("optgroup",{key:B,label:B},[(ye(!0),be(yn,null,bn(T,se=>(ye(),be("option",{key:se.id,value:se.id},Ne(se.title),9,Zb))),128))],8,Qb))),128)),Q("option",{value:wn(Wn).id},Ne(wn(Wn).title),9,ew)],40,Xb),Q("button",{class:"pg-btn pg-primary",disabled:h.value.state!=="ready"||f.value!=="",title:`Run main (${wn(ge)}+Enter)`,onClick:E},Ne(f.value==="run"?"Running…":"Run"),9,tw),Q("button",{class:"pg-btn",disabled:h.value.state!=="ready"||f.value!=="",title:`Run the tests (${wn(ge)}+Shift+Enter)`,onClick:$},Ne(f.value==="test"?"Testing…":"Test"),9,nw),f.value?(ye(),be("button",{key:0,class:"pg-btn",title:"Stop the program",onClick:V},"Stop")):vn("",!0),Q("button",{class:"pg-btn",disabled:h.value.state!=="ready",title:`Format the file (${wn(ge)}+S)`,onClick:K},"Format",8,iw),Q("button",{class:"pg-btn",title:"Copy a link that holds the whole program",onClick:ke},"Share"),Q("label",sw,[k[5]||(k[5]=Q("span",null,"Arguments",-1)),Rs(Q("input",{"onUpdate:modelValue":k[1]||(k[1]=T=>a.value=T),placeholder:"Ann Bo",spellcheck:"false",onKeydown:gd(E,["enter"])},null,544),[[md,a.value]])]),Q("span",{class:kn(["pg-status",{failed:h.value.state==="failed"}]),role:"status"},Ne(p.value||y.value),3)]),Q("div",rw,[Q("div",ow,[Q("div",aw,[(ye(!0),be(yn,null,bn(m.value,T=>(ye(),be("div",{key:T,class:kn(["pg-tab",{active:T===o.value}]),role:"tab","aria-selected":T===o.value,onClick:B=>I(T),onDblclick:B=>Ue(T)},[Q("span",null,Ne(T),1),m.value.length>1?(ye(),be("button",{key:0,class:"pg-close","aria-label":`Delete ${T}`,onClick:yd(B=>Me(T),["stop"])},"×",8,hw)):vn("",!0)],42,lw))),128)),Q("button",{class:"pg-tab pg-add",title:"Add a file","aria-label":"Add a file",onClick:re},"+")]),Q("div",{ref_key:"host",ref:e,class:"pg-code"},null,512)]),Q("div",cw,[Q("div",dw,[Q("button",{class:kn(["pg-tab",{active:u.value==="output"}]),role:"tab",onClick:k[2]||(k[2]=T=>u.value="output")},"Output",2),Q("button",{class:kn(["pg-tab",{active:u.value==="problems"}]),role:"tab",onClick:k[3]||(k[3]=T=>u.value="problems")},[k[6]||(k[6]=Ro(" Problems",-1)),d.value.length?(ye(),be("span",uw,Ne(d.value.length),1)):vn("",!0)],2)]),Rs(Q("pre",fw,[(ye(!0),be(yn,null,bn(c.value,(T,B)=>(ye(),be("span",{key:B,class:kn(`pg-${T.kind}`)},Ne(T.text),3))),128))],512),[[Eo,u.value==="output"]]),Rs(Q("ul",pw,[d.value.length?vn("",!0):(ye(),be("li",mw,"No problems.")),(ye(!0),be(yn,null,bn(d.value,(T,B)=>(ye(),be("li",{key:B,onClick:se=>Oe(T)},[T.path?(ye(),be("span",yw,Ne(T.path)+":"+Ne(T.range.start.line+1)+":"+Ne(T.range.start.character+1),1)):vn("",!0),Ro(" "+Ne(T.message),1)],8,gw))),128))],512),[[Eo,u.value==="problems"]])])])]))}});export{kw as default};
