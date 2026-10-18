import 'package:flutter/material.dart';

import '../api.dart';
import 'palette.dart';

/// Překryv "Spojení se serverem ztraceno" – poslední data zůstávají vidět pod ním.
class ConnectionOverlay extends StatelessWidget {
  const ConnectionOverlay({super.key, required this.client});

  final PredavkaClient client;

  @override
  Widget build(BuildContext context) {
    return ListenableBuilder(
      listenable: client,
      builder: (context, _) {
        final show = client.stale;
        return IgnorePointer(
          ignoring: !show,
          child: AnimatedOpacity(
            opacity: show ? 1 : 0,
            duration: const Duration(milliseconds: 800),
            child: Container(
              color: Palette.background.withValues(alpha: 0.55),
              alignment: Alignment.center,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 36, vertical: 24),
                decoration: BoxDecoration(
                  color: Palette.surfaceHigh,
                  borderRadius: BorderRadius.circular(20),
                  border: Border.all(color: Palette.late.withValues(alpha: 0.5), width: 2),
                ),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Text(
                      'Spojení se serverem ztraceno',
                      textAlign: TextAlign.center,
                      style: TextStyle(fontSize: 32, fontWeight: FontWeight.w600, color: Palette.text),
                    ),
                    const SizedBox(height: 8),
                    Text(
                      client.snapshot == null
                          ? 'Zkouším se připojit k ${client.baseUrl}'
                          : 'Zobrazuji poslední známý stav. Zkouším se znovu připojit…',
                      textAlign: TextAlign.center,
                      style: const TextStyle(fontSize: 18, color: Palette.textDim),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );
  }
}
