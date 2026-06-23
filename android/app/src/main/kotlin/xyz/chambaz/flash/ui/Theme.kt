package xyz.chambaz.flash.ui

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

val LocalAccent = staticCompositionLocalOf { Color(0xFF5A83E9) }

val CurrentAccent: Color
    @Composable
    @ReadOnlyComposable
    get() = LocalAccent.current

val accentColors = listOf(
    Color(0xFFD5632A),
    Color(0xFFC67200),
    Color(0xFF719905),
    Color(0xFF07A459),
    Color(0xFF00A387),
    Color(0xFF0099CB),
    Color(0xFF5A83E9),
    Color(0xFF9570DC),
    Color(0xFFC45EAA),
)

@Composable
fun FlashTheme(theme: String = "black", accentIndex: Int = 6, content: @Composable () -> Unit) {
    val accent = accentColors.getOrElse(accentIndex) { accentColors[6] }
    val colors = when (theme) {
        "dark" -> darkColorScheme(
            primary = accent,
            onPrimary = Color.White,
            secondary = accent,
            onSecondary = Color.White,
            background = Color(0xFF222222),
            onBackground = Color.White,
            surface = Color(0xFF2E2E2E),
            onSurface = Color.White,
            surfaceVariant = Color(0xFF3A3A3A),
            onSurfaceVariant = Color(0xFFAAAAAA),
            surfaceTint = Color.Transparent,
            outline = Color(0xFF555555),
            outlineVariant = Color(0xFF333333),
            error = Color(0xFFC1454F),
            onError = Color.White,
        )
        "light" -> lightColorScheme(
            primary = accent,
            onPrimary = Color.White,
            secondary = accent,
            onSecondary = Color.White,
            background = Color.White,
            onBackground = Color(0xFF111111),
            surface = Color(0xFFF0F0F0),
            onSurface = Color(0xFF111111),
            surfaceVariant = Color(0xFFE5E5E5),
            onSurfaceVariant = Color(0xFF666666),
            surfaceTint = Color.Transparent,
            outline = Color(0xFF999999),
            outlineVariant = Color(0xFFCCCCCC),
            error = Color(0xFFC1454F),
            onError = Color.White,
        )
        else -> darkColorScheme(
            primary = accent,
            onPrimary = Color.White,
            secondary = accent,
            onSecondary = Color.White,
            background = Color.Black,
            onBackground = Color.White,
            surface = Color(0xFF0D0D0D),
            onSurface = Color.White,
            surfaceVariant = Color(0xFF1A1A1A),
            onSurfaceVariant = Color(0xFFBBBBBB),
            surfaceTint = Color.Transparent,
            outline = Color(0xFF444444),
            outlineVariant = Color(0xFF222222),
            error = Color(0xFFC1454F),
            onError = Color.White,
        )
    }
    CompositionLocalProvider(LocalAccent provides accent) {
        MaterialTheme(colorScheme = colors, content = content)
    }
}
