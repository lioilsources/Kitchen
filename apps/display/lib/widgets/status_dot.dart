import 'package:flutter/material.dart';

import '../api.dart';
import '../l10n/gen/app_localizations.dart';
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
        final l = AppLocalizations.of(context);
        final (color, label) = switch (client.state) {
          _ when client.stale => (Palette.late, l.connStale),
          ConnState.connected => (Palette.ok, l.connConnected),
          ConnState.connecting => (Palette.warn, l.connConnecting),
          ConnState.disconnected => (Palette.late, l.connDisconnected),
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
