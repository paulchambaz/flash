package xyz.chambaz.flash.ui

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.foundation.verticalScroll
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import kotlinx.coroutines.launch
import kotlin.math.exp
import kotlin.math.ln

private val STEP_MIN_MS = 60_000L
private val STEP_MAX_MS = 31_536_000_000L

private fun msToPos(ms: Long): Float =
    ((ln(ms.toDouble()) - ln(STEP_MIN_MS.toDouble())) /
     (ln(STEP_MAX_MS.toDouble()) - ln(STEP_MIN_MS.toDouble()))).toFloat()

private fun posToMs(pos: Float): Long {
    val lnMin = ln(STEP_MIN_MS.toDouble())
    val lnMax = ln(STEP_MAX_MS.toDouble())
    return exp(lnMin + pos * (lnMax - lnMin)).toLong()
}

private fun snapStep(ms: Long): Long {
    val snapPoints = longArrayOf(
        60_000, 300_000, 600_000, 1_800_000,
        3_600_000, 21_600_000, 86_400_000, 259_200_000,
        604_800_000, 1_209_600_000,
        2_592_000_000, 5_184_000_000, 7_776_000_000,
        15_552_000_000, 31_536_000_000,
    )
    val pos = msToPos(ms)
    return snapPoints.minByOrNull { kotlin.math.abs(msToPos(it) - pos) }
        ?.takeIf { kotlin.math.abs(msToPos(it) - pos) < 0.02f } ?: ms
}

private fun formatStep(ms: Long): String = when {
    ms < 3_600_000L        -> "${ms / 60_000} min"
    ms < 86_400_000L       -> "${ms / 3_600_000} h"
    ms < 604_800_000L      -> "${ms / 86_400_000} d"
    ms < 2_592_000_000L    -> "${ms / 604_800_000} w"
    ms < 31_536_000_000L   -> "${ms / 2_592_000_000} mo"
    else                   -> "${ms / 31_536_000_000} y"
}

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun SettingsScreen(
    initialUrl: String,
    initialToken: String,
    initialTheme: String,
    initialAccentIndex: Int,
    initialThreshold: Float,
    initialStep: Long,
    onBack: () -> Unit,
    onSave: suspend (url: String, token: String) -> Unit,
    onThemeChange: (String) -> Unit,
    onAccentChange: (Int) -> Unit,
    onThresholdChange: (Float) -> Unit,
    onStepChange: (Long) -> Unit,
) {
    val focusManager = LocalFocusManager.current
    val isImeVisible = WindowInsets.isImeVisible
    LaunchedEffect(isImeVisible) { if (!isImeVisible) focusManager.clearFocus() }
    BackHandler { onBack() }
    BackHandler(enabled = isImeVisible) { focusManager.clearFocus() }

    var url by remember { mutableStateOf(initialUrl) }
    var token by remember { mutableStateOf(initialToken) }
    var theme by remember { mutableStateOf(initialTheme) }
    var accentIndex by remember { mutableIntStateOf(initialAccentIndex) }
    var threshold by remember { mutableFloatStateOf(initialThreshold) }
    var stepMs by remember { mutableLongStateOf(initialStep) }
    var loading by remember { mutableStateOf(false) }
    var error by remember { mutableStateOf<String?>(null) }
    var showSaved by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()

    fun save() {
        if (url.isBlank() || token.isBlank()) {
            error = "URL and token required"
            return
        }
        focusManager.clearFocus()
        scope.launch {
            loading = true
            error = null
            showSaved = false
            try {
                onSave(url.trim(), token.trim())
                showSaved = true
            } catch (e: Exception) {
                error = when {
                    e.message?.contains("401") == true -> "Invalid token"
                    e.message?.contains("Unable to resolve") == true ||
                    e.message?.contains("Failed to connect") == true -> "Server unreachable"
                    else -> "Error: ${e.message}"
                }
            } finally {
                loading = false
            }
        }
    }

    Scaffold(
        containerColor = MaterialTheme.colorScheme.background,
        topBar = {
            TopAppBar(
                title = { Text("Settings") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, "Back")
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background,
                    titleContentColor = MaterialTheme.colorScheme.onBackground,
                    navigationIconContentColor = MaterialTheme.colorScheme.onBackground,
                ),
            )
        },
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = 16.dp)
                .verticalScroll(rememberScrollState())
                .pointerInput(Unit) { detectTapGestures { focusManager.clearFocus() } },
        ) {
            Spacer(Modifier.height(8.dp))

            // ── Apparence ────────────────────────────────────────────────────
            Text("Appearance", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(12.dp))

            Text("Theme", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onBackground)
            Spacer(Modifier.height(8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                listOf("Light", "Dark", "Black").forEach { option ->
                    val selected = theme == option.lowercase()
                    OutlinedButton(
                        onClick = {
                            theme = option.lowercase()
                            onThemeChange(theme)
                        },
                        border = BorderStroke(1.dp, if (selected) CurrentAccent else Color(0xFF444444)),
                        colors = ButtonDefaults.outlinedButtonColors(
                            containerColor = if (selected) CurrentAccent.copy(alpha = 0.15f) else Color.Transparent,
                        ),
                        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 6.dp),
                    ) {
                        Text(option, color = if (selected) CurrentAccent else MaterialTheme.colorScheme.onSurfaceVariant)
                    }
                }
            }

            Spacer(Modifier.height(16.dp))

            Text("Accent color", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onBackground)
            Spacer(Modifier.height(8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                accentColors.forEachIndexed { i, color ->
                    Box(
                        modifier = Modifier
                            .size(32.dp)
                            .clip(CircleShape)
                            .background(color)
                            .then(
                                if (accentIndex == i)
                                    Modifier.border(2.dp, MaterialTheme.colorScheme.onBackground, CircleShape)
                                else
                                    Modifier
                            )
                            .clickable {
                                accentIndex = i
                                onAccentChange(i)
                            }
                    )
                }
            }

            Spacer(Modifier.height(24.dp))
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Spacer(Modifier.height(24.dp))

            // ── Review ────────────────────────────────────────────────────────
            Text("Review", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(12.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("Success threshold", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onBackground)
                Text("%.2f".format(threshold), style = MaterialTheme.typography.bodyMedium, color = CurrentAccent)
            }
            Slider(
                value = threshold,
                onValueChange = { v ->
                    val snapped = if (kotlin.math.abs(v - 0.7f) < 0.03f) 0.7f else v
                    threshold = snapped
                    onThresholdChange(snapped)
                },
                valueRange = 0f..1f,
                colors = SliderDefaults.colors(thumbColor = CurrentAccent, activeTrackColor = CurrentAccent),
            )

            Spacer(Modifier.height(12.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("Pace", style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onBackground)
                Text(formatStep(stepMs), style = MaterialTheme.typography.bodyMedium, color = CurrentAccent)
            }
            Slider(
                value = msToPos(stepMs),
                onValueChange = { pos ->
                    val snapped = snapStep(posToMs(pos))
                    stepMs = snapped
                    onStepChange(snapped)
                },
                valueRange = 0f..1f,
                colors = SliderDefaults.colors(thumbColor = CurrentAccent, activeTrackColor = CurrentAccent),
            )

            Spacer(Modifier.height(24.dp))
            HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
            Spacer(Modifier.height(24.dp))

            // ── Serveur ───────────────────────────────────────────────────────
            Text("Server", style = MaterialTheme.typography.labelMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(Modifier.height(12.dp))

            OutlinedTextField(
                value = url,
                onValueChange = { url = it; showSaved = false },
                label = { Text("URL") },
                placeholder = { Text("https://flash.example.com") },
                modifier = Modifier.fillMaxWidth(),
                enabled = !loading,
                singleLine = true,
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next),
            )
            Spacer(Modifier.height(12.dp))
            OutlinedTextField(
                value = token,
                onValueChange = { token = it; showSaved = false },
                label = { Text("Token") },
                visualTransformation = PasswordVisualTransformation(),
                modifier = Modifier.fillMaxWidth(),
                enabled = !loading,
                singleLine = true,
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(onDone = { save() }),
            )

            if (error != null) {
                Spacer(Modifier.height(8.dp))
                Text(error!!, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
            }
            if (showSaved) {
                Spacer(Modifier.height(8.dp))
                Text("Connected successfully", color = CurrentAccent, style = MaterialTheme.typography.bodySmall)
            }

            Spacer(Modifier.height(16.dp))
            Button(
                onClick = { save() },
                modifier = Modifier.fillMaxWidth(),
                enabled = !loading,
            ) {
                if (loading) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(18.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onPrimary,
                    )
                } else {
                    Text("Test and save")
                }
            }

            Spacer(Modifier.height(24.dp))
        }
    }
}
