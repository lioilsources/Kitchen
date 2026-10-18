import 'dart:async';

import 'package:flutter/material.dart';

import '../api.dart';
import 'palette.dart';

/// Poslední hláška serveru jako klidný titulek dole (~8 s).
class AnnounceCaption extends StatefulWidget {
  const AnnounceCaption({
    super.key,
    required this.client,
    this.fontSize = 28,
    this.visibleFor = const Duration(seconds: 8),
  });

  final PredavkaClient client;
  final double fontSize;
  final Duration visibleFor;

  @override
  State<AnnounceCaption> createState() => _AnnounceCaptionState();
}

class _AnnounceCaptionState extends State<AnnounceCaption> {
  int _seenSeq = 0;
  String _text = '';
  bool _visible = false;
  Timer? _hide;

  @override
  void initState() {
    super.initState();
    _seenSeq = widget.client.announceSeq; // staré hlášky po startu neukazovat
    widget.client.addListener(_onClient);
  }

  @override
  void didUpdateWidget(AnnounceCaption old) {
    super.didUpdateWidget(old);
    if (old.client != widget.client) {
      old.client.removeListener(_onClient);
      _seenSeq = widget.client.announceSeq;
      widget.client.addListener(_onClient);
    }
  }

  void _onClient() {
    final c = widget.client;
    if (c.announceSeq == _seenSeq) return;
    _seenSeq = c.announceSeq;
    final text = c.lastAnnounce?.text ?? '';
    if (text.isEmpty) return;
    setState(() {
      _text = text;
      _visible = true;
    });
    _hide?.cancel();
    _hide = Timer(widget.visibleFor, () {
      if (mounted) setState(() => _visible = false);
    });
  }

  @override
  void dispose() {
    _hide?.cancel();
    widget.client.removeListener(_onClient);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: AnimatedOpacity(
        opacity: _visible ? 1 : 0,
        duration: const Duration(milliseconds: 500),
        child: Container(
          margin: const EdgeInsets.fromLTRB(24, 0, 24, 20),
          padding: const EdgeInsets.symmetric(horizontal: 28, vertical: 14),
          decoration: BoxDecoration(
            color: Palette.surfaceHigh.withValues(alpha: 0.95),
            borderRadius: BorderRadius.circular(16),
          ),
          child: Text(
            _text,
            textAlign: TextAlign.center,
            maxLines: 2,
            overflow: TextOverflow.ellipsis,
            style: TextStyle(fontSize: widget.fontSize, color: Palette.text, height: 1.2),
          ),
        ),
      ),
    );
  }
}
