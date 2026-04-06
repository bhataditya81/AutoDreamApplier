/**
 * Manual Jest mock for framer-motion.
 *
 * Strategy:
 *   - useMotionValue stores its value in useState so that calling .set()
 *     triggers a genuine React state update (guaranteed to flush inside
 *     React Testing Library's act() wrapper).
 *   - The motion value exposes _val (the current React state) so useTransform
 *     can derive synchronously on every render without subscriptions.
 *   - animate() calls mv.set(target) immediately — no rAF, no tween.
 *   - motion.* elements are plain HTML tags; animation/gesture props are stripped
 *     and MotionValue children are unwrapped via .get().
 *   - AnimatePresence just renders children.
 */

var React = require('react');

// ---------------------------------------------------------------------------
// useMotionValue
// ---------------------------------------------------------------------------
function useMotionValue(initial) {
  var stateArr = React.useState(initial);
  var val = stateArr[0];
  var setVal = stateArr[1];

  // Keep a ref so .get() always returns the latest value synchronously.
  var valRef = React.useRef(val);
  valRef.current = val;

  // Keep a ref to the latest setter so .set() never closes over a stale one.
  var setRef = React.useRef(setVal);
  setRef.current = setVal;

  // Return a new object each render; _val is the reactive state so that
  // useTransform can re-derive whenever the parent re-renders.
  return {
    _val: val,
    get: function () { return valRef.current; },
    set: function (v) { valRef.current = v; setRef.current(v); },
  };
}

// ---------------------------------------------------------------------------
// useTransform
// ---------------------------------------------------------------------------
function useTransform(mv, fn) {
  // mv._val is the React state value — reading it here makes the component
  // that calls useTransform re-execute this line whenever mv re-renders,
  // so the derived value is always fresh.
  var derived = fn(mv._val);
  return { get: function () { return derived; } };
}

// ---------------------------------------------------------------------------
// animate — immediately set the target value
// ---------------------------------------------------------------------------
function animate(mv, target) {
  if (mv && typeof mv.set === 'function') mv.set(target);
  return { stop: function () {} };
}

// ---------------------------------------------------------------------------
// makeMotionComponent — strips animation props, unwraps MotionValue children
// ---------------------------------------------------------------------------
var STRIP_PROPS = new Set([
  'initial', 'animate', 'exit', 'variants', 'transition',
  'whileHover', 'whileTap', 'whileFocus', 'whileDrag', 'whileInView',
  'layout', 'layoutId', 'drag', 'dragConstraints', 'dragElastic',
  'onAnimationStart', 'onAnimationComplete', 'onDragStart', 'onDragEnd',
  'viewport', 'custom',
]);

function makeMotionComponent(tag) {
  var MotionElement = React.forwardRef(function (props, ref) {
    var filtered = {};
    Object.keys(props).forEach(function (key) {
      if (!STRIP_PROPS.has(key)) {
        filtered[key] = props[key];
      }
    });
    filtered.ref = ref;

    var children = filtered.children;
    if (
      children !== null &&
      children !== undefined &&
      typeof children === 'object' &&
      typeof children.get === 'function'
    ) {
      filtered.children = children.get();
    }

    return React.createElement(tag, filtered);
  });
  MotionElement.displayName = 'motion.' + tag;
  return MotionElement;
}

// ---------------------------------------------------------------------------
// AnimatePresence
// ---------------------------------------------------------------------------
function AnimatePresence(props) {
  return props.children || null;
}

// ---------------------------------------------------------------------------
// exports
// ---------------------------------------------------------------------------
module.exports = {
  motion: {
    span:    makeMotionComponent('span'),
    div:     makeMotionComponent('div'),
    button:  makeMotionComponent('button'),
    svg:     makeMotionComponent('svg'),
    aside:   makeMotionComponent('aside'),
    section: makeMotionComponent('section'),
    header:  makeMotionComponent('header'),
    footer:  makeMotionComponent('footer'),
    nav:     makeMotionComponent('nav'),
    main:    makeMotionComponent('main'),
    p:       makeMotionComponent('p'),
    ul:      makeMotionComponent('ul'),
    li:      makeMotionComponent('li'),
    a:       makeMotionComponent('a'),
    h1:      makeMotionComponent('h1'),
    h2:      makeMotionComponent('h2'),
    h3:      makeMotionComponent('h3'),
  },
  AnimatePresence: AnimatePresence,
  useMotionValue:  useMotionValue,
  useTransform:    useTransform,
  animate:         animate,
};
