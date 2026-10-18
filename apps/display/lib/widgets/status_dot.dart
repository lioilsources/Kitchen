import 'package:flutter/material.dart';

import '../api.dart';
import 'palette.dart';

/// Malá tečka stavu spojení.
class StatusDot extends StatelessWidget {
  const StatusDot({super.key, required this.client});

  final PredavkaClient client;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: client,
      builder: (context, _) {
        final (color, label) = switch (client.state) {
          _ when client.stale => (Palette.late, 'Spojení ztraceno'),
          ConnState.connected => (Palette.ok, 'Připojeno'),
          ConnState.connecting => (Palette.warn, 'Připojuji…'),
          ConnState.disconnected => (Palette.late, 'Odpojeno'),
        };
        return Tooltip(
          message: label,
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 600),
            width: 12,
            height: 12,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
          ),
        );
      },
    );
  }
}
