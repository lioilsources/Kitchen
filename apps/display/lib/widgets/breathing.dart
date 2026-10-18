import 'package:flutter/material.dart';

/// Pomalé "dýchání" barvy pod obsahem (perioda ~2,6 s). Nikdy neblikne:
/// průhlednost plynule osciluje mezi [minAlpha] a [maxAlpha].
class Breathing extends StatefulWidget {
  const Breathing({
    super.key,
    required this.color,
    required this.child,
    this.baseColor,
    this.borderRadius = const BorderRadius.all(Radius.circular(16)),
    this.minAlpha = 0.10,
    this.maxAlpha = 0.34,
    this.period = const Duration(milliseconds: 2600),
  });

  /// null = bez dýchání (jen [baseColor]).
  final Color? color;
  final Color? baseColor;
  final Widget child;
  final BorderRadius borderRadius;
  final double minAlpha;
  final double maxAlpha;
  final Duration period;

  @override
  State<Breathing> createState() => _BreathingState();
}

class _BreathingState extends State<Breathing> with SingleTickerProviderStateMixin {
  late final AnimationController _c = AnimationController(
    vsync: this,
    duration: Duration(milliseconds: widget.period.inMilliseconds ~/ 2),
  );
  late final Animation<double> _t = CurvedAnimation(parent: _c, curve: Curves.easeInOut);

  @override
  void initState() {
    super.initState();
    _sync();
  }

  @override
  void didUpdateWidget(Breathing old) {
    super.didUpdateWidget(old);
    _sync();
  }

  void _sync() {
    if (widget.color != null) {
      if (!_c.isAnimating) _c.repeat(reverse: true);
    } else {
      _c.stop();
    }
  }

  @override
  void dispose() {
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final color = widget.color;
    final base = widget.baseColor ?? Colors.transparent;
    if (color == null) {
      return DecoratedBox(
        decoration: BoxDecoration(color: base, borderRadius: widget.borderRadius),
        child: widget.child,
      );
    }
    return AnimatedBuilder(
      animation: _t,
      child: widget.child,
      builder: (context, child) {
        final a = widget.minAlpha + (widget.maxAlpha - widget.minAlpha) * _t.value;
        return DecoratedBox(
          decoration: BoxDecoration(
            color: Color.alphaBlend(color.withValues(alpha: a), base),
            borderRadius: widget.borderRadius,
            border: Border.all(color: color.withValues(alpha: 0.35 + a), width: 2),
          ),
          child: child,
        );
      },
    );
  }
}
