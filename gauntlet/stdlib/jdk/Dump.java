// Dump.java — the JVM's exported API as a manifest, for gauntlet/stdlib/jvm.go.
//
// Go ships `$GOROOT/api/go1*.txt`, an official machine-readable list of its
// exported surface. The JDK ships no such file and does not need one: THE
// RUNTIME IS THE MANIFEST. `jrt:/` enumerates every class in every module and
// reflection reads its signature, so this dump is not a description of the JDK
// that could drift from it — it is the JDK, asked.
//
// That is the coercion result's rule arriving on a second host: the host is the
// oracle. Where Go's survey has to trust a text file, this one cannot be wrong
// about what exists.
//
// TWO THINGS IT DELIBERATELY DOES.
//
// GENERIC SIGNATURES, not erased ones. `getParameterTypes` gives `java.util.List`
// and `getGenericParameterTypes` gives `java.util.List<java.lang.String>`. The
// erased form is what the JVM verifies and the generic form is what a Java
// SOURCE declaration must spell, and a target template is source. Reading the
// erased form would report a capability we do not have, because a raw `List`
// hands back `Object`.
//
// EXPORTED PACKAGES ONLY, unqualified. A package a module exports to one named
// module is not part of the JDK's public surface, and counting it would inflate
// the denominator with names no program may call.
//
// Everything is SORTED, because a manifest that reorders itself makes every
// downstream "byte-identical" claim meaningless (backend-2026-09-06).
//
//   java gauntlet/stdlib/jdk/Dump.java > /tmp/jdk-api.txt

import java.lang.reflect.*;
import java.nio.file.*;
import java.util.*;
import java.util.stream.*;

public class Dump {
    public static void main(String[] args) throws Exception {
        // Which packages are exported to everybody.
        Set<String> exported = new TreeSet<>();
        Map<String, String> moduleOf = new TreeMap<>();
        for (Module m : ModuleLayer.boot().modules()) {
            for (java.lang.module.ModuleDescriptor.Exports e : m.getDescriptor().exports()) {
                if (!e.isQualified()) {
                    exported.add(e.source());
                    moduleOf.put(e.source(), m.getName());
                }
            }
        }

        FileSystem fs = FileSystems.getFileSystem(java.net.URI.create("jrt:/"));
        List<String> lines = new ArrayList<>();
        Set<String> seenTypes = new TreeSet<>();

        for (String pkg : exported) {
            String mod = moduleOf.get(pkg);
            Path dir = fs.getPath("/modules", mod, pkg.replace('.', '/'));
            if (!Files.isDirectory(dir)) continue;
            List<String> names = new ArrayList<>();
            try (DirectoryStream<Path> ds = Files.newDirectoryStream(dir)) {
                for (Path p : ds) {
                    String f = p.getFileName().toString();
                    if (f.endsWith(".class")) names.add(f.substring(0, f.length() - 6));
                }
            } catch (Exception e) { continue; }
            Collections.sort(names);
            for (String n : names) {
                Class<?> c;
                try { c = Class.forName(pkg + "." + n, false, ClassLoader.getSystemClassLoader()); }
                catch (Throwable t) { continue; }
                if (!Modifier.isPublic(c.getModifiers())) continue;
                if (c.isSynthetic() || c.isAnonymousClass() || c.isLocalClass()) continue;
                String cn = c.getCanonicalName();
                if (cn == null) continue;
                dumpClass(lines, seenTypes, pkg, c);
            }
        }
        Collections.sort(lines);
        StringBuilder out = new StringBuilder();
        for (String l : lines) out.append(l).append('\n');
        System.out.print(out);
    }

    static void dumpClass(List<String> lines, Set<String> seen, String pkg, Class<?> c) {
        String cn = c.getCanonicalName();
        if (cn == null || !seen.add(cn)) return;
        String kind = c.isInterface() ? "interface"
                    : c.isEnum() ? "enum"
                    : Modifier.isAbstract(c.getModifiers()) ? "abstract" : "class";
        // A CLASS WITH TYPE PARAMETERS is not a type until it is applied, and our
        // type language has no way to apply one. Recorded so the survey can count
        // it rather than discover it as a parse failure.
        String tp = c.getTypeParameters().length > 0 ? " generic" : "";
        // A FUNCTIONAL INTERFACE is one abstract method, and on this host that
        // is an ORDINARY OBJECT: holding one costs nothing and calling it is a
        // method call. Only MANUFACTURING one is callbacks.md tier 3. Marked so
        // the survey can count the distinction rather than assume Go's, where a
        // `func(...)` value is refused in both directions.
        lines.add("pkg " + pkg + ", type " + cn + " " + kind + tp + (functional(c) ? " functional" : ""));

        // THE SUBTYPING RELATION, WHICH THIS HOST DECLARES.
        //
        // Go's survey has to COMPUTE `T implements I` structurally and then let
        // `go build` refuse the 182 candidates that were false, because the api
        // manifest lists the exported API and an interface sealed by an
        // unexported method looks satisfied by everything. The JVM has no such
        // problem: `extends` and `implements` are written down, so the relation
        // is READ rather than derived and there is nothing for a host filter to
        // catch. Direct edges only — the loader closes them transitively, or a
        // target file would be stating a closure by hand and getting it wrong.
        List<String> sup = new ArrayList<>();
        try {
            Class<?> sc = c.getSuperclass();
            if (sc != null && sc != Object.class && sc.getCanonicalName() != null) sup.add(sc.getCanonicalName());
            for (Class<?> i : c.getInterfaces())
                if (i.getCanonicalName() != null) sup.add(i.getCanonicalName());
        } catch (Throwable t) { }
        if (!sup.isEmpty()) {
            Collections.sort(sup);
            lines.add("pkg " + pkg + ", extends " + cn + " " + String.join(" ", sup));
        }

        try {
            for (Constructor<?> k : c.getConstructors()) {
                if (k.isSynthetic()) continue;
                lines.add("pkg " + pkg + ", ctor " + cn + "(" + params(k.getGenericParameterTypes(), k.isVarArgs()) + ")"
                        + (k.getTypeParameters().length > 0 ? " generic" : ""));
            }
            for (Method m : c.getMethods()) {
                if (m.isSynthetic() || m.isBridge()) continue;
                if (m.getDeclaringClass() == Object.class) continue;
                // A method is listed on the class that DECLARES it; an inherited
                // one is reachable through the subclass too, but counting it on
                // both would count one name twice.
                if (m.getDeclaringClass() != c) continue;
                String sig = m.getName() + "(" + params(m.getGenericParameterTypes(), m.isVarArgs()) + ") "
                           + m.getGenericReturnType().getTypeName()
                           + (m.getTypeParameters().length > 0 ? " generic" : "");
                lines.add("pkg " + pkg + ", " + (Modifier.isStatic(m.getModifiers()) ? "func " : "method ")
                        + (Modifier.isStatic(m.getModifiers()) ? cn + "." : "(" + cn + ") ") + sig);
            }
            for (Field f : c.getFields()) {
                if (f.isSynthetic() || f.getDeclaringClass() != c) continue;
                lines.add("pkg " + pkg + ", " + (Modifier.isStatic(f.getModifiers()) ? "static " : "field ")
                        + cn + "." + f.getName() + " " + f.getGenericType().getTypeName());
            }
        } catch (Throwable t) {
            // A class whose signature mentions something the module graph does
            // not resolve is skipped rather than guessed at.
        }
    }

    static boolean functional(Class<?> c) {
        if (!c.isInterface()) return false;
        int n = 0;
        for (Method m : c.getMethods()) {
            if (!Modifier.isAbstract(m.getModifiers()) || m.isSynthetic()) continue;
            try { Object.class.getMethod(m.getName(), m.getParameterTypes()); continue; }
            catch (NoSuchMethodException ignored) { }
            n++;
        }
        return n == 1;
    }

    static String params(Type[] ts, boolean varargs) {
        StringBuilder b = new StringBuilder();
        for (int i = 0; i < ts.length; i++) {
            if (i > 0) b.append(", ");
            String t = ts[i].getTypeName();
            if (varargs && i == ts.length - 1) t = "..." + t;
            b.append(t);
        }
        return b.toString();
    }
}
