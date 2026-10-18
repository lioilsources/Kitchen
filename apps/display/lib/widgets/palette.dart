import 'package:flutter/material.dart';

import '../models.dart';

/// Klidná paleta: neutrální tmavé pozadí, tlumené akcenty.
abstract final class Palette {
  static const background = Color(0xFF111315);
  static const surface = Color(0xFF1C1F23);
  static const surfaceHigh = Color(0xFF252A2F);
  static const outline = Color(0xFF333A41);
  static const text = Color(0xFFE6E8EA);
  static const textDim = Color(0xFF9AA2AA);
  static const textFaint = Color(0xFF5F676F);

  static const newItem = Color(0xFF7FA6C9); // jemná modrošedá
  static const warn = Color(0xFFE3A15F); // měkká oranžová
  static const late = Color(0xFFD97A72); // měkká červená
  static const ready = Color(0xFF86C49A); // měkká zelená
  static const ok = Color(0xFF6FBF8A);

  static Color? alertColor(Alert a) => switch (a) {
    Alert.warn => warn,
    Alert.late => late,
    Alert.none => null,
  };
}
